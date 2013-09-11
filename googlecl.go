// TODO: Handle user auth.
// TODO: Verify service account auth actually works.
// TODO: Cache discovery/directory documents for faster requests.
// TODO: Handle media upload/download.
// TODO: Handle repeated parameters.

package main

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"code.google.com/p/goauth2/oauth/jwt"
)

var (
	// Flags that get parsed before the command, necessary for loading Cloud Endpoints APIs
	// e.g., "googlecl --endpoint=foo help myapi" parses the endpoint flag before loading the API
	endpointFs   = flag.NewFlagSet("endpoint", flag.ExitOnError)
	flagEndpoint = endpointFs.String("endpoint", "https://www.googleapis.com/", "Cloud Endpoints URL, e.g., https://my-app-id.appspot.com/_ah/api/")

	// Flags that get parsed after the command, common to all APIs
	flagset     = flag.NewFlagSet("googlecl", flag.ExitOnError)
	flagPem     = flagset.String("meta.pem", "", "Location of .pem file")
	flagSecrets = flagset.String("meta.secrets", "", "Location of client_secrets.json")
	flagStdin   = flagset.Bool("meta.in", false, "Accept request body from stdin")
	flagInFile  = flagset.String("meta.inFile", "", "File to pass as request body")
)

func simpleHelp() {
	fmt.Println("Makes requests to Google APIs")
	fmt.Println("Usage:")
	fmt.Println("  googlecl <api> <method> --param=foo")
}

func help() {
	args := endpointFs.Args()
	nargs := len(args)
	if nargs == 0 || (nargs == 1 && args[0] == "help") {
		simpleHelp()
		return
	}
	apiName := args[1]
	api, err := loadAPI(apiName)
	if err != nil {
		log.Fatal(err)
	}
	if nargs == 2 {
		// googlecl help <api>
		fmt.Println(api.Title, api.Description)
		fmt.Println("More information:", api.DocumentationLink)
		fmt.Println("Methods:")
		for _, m := range api.Methods {
			fmt.Println(m.ID, m.Description)
		}
		type pair struct {
			k string
			r Resource
		}
		l := []pair{}
		for k, r := range api.Resources {
			l = append(l, pair{k, r})
		}
		for i := 0; i < len(l); i++ {
			r := l[i].r
			for _, m := range r.Methods {
				fmt.Printf("%s - %s\n", m.ID[len(api.Name)+1:], m.Description)
			}
			for k, r := range r.Resources {
				l = append(l, pair{k, r})
			}
		}
	} else {
		// googlecl help <api> <method>
		method := args[2]
		m := findMethod(method, *api)
		fmt.Println(method, m.Description)
		fmt.Println("Parameters:")
		for k, p := range m.Parameters {
			fmt.Printf("  --%s (%s) - %s\n", k, p.Type, p.Description)
		}
		for k, p := range api.Parameters {
			fmt.Printf("  --%s (%s) - %s\n", k, p.Type, p.Description)
		}
	}
}

func list() {
	var directory struct {
		Items []struct {
			Name, Version, Description string
		}
	}
	getAndParse("discovery/v1/apis", &directory)
	fmt.Println("Available methods:")
	for _, i := range directory.Items {
		fmt.Printf("%s %s - %s\n", i.Name, i.Version, i.Description)
	}
}

func main() {
	endpointFs.Parse(os.Args[1:])
	if len(endpointFs.Args()) == 0 {
		simpleHelp()
		return
	}

	cmd := endpointFs.Args()[0]
	if cmd == "help" {
		help()
		return
	} else if cmd == "list" {
		list()
		return
	}

	method := endpointFs.Args()[1]
	if method == "" {
		log.Fatal("Must specify API method to call")
	}

	api, err := loadAPI(cmd)
	if err != nil {
		log.Fatal(err)
	}
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", cmd)
	}

	m := findMethod(method, *api)
	for k, p := range api.Parameters {
		flagset.String(k, p.Default, p.Description)
	}
	for k, p := range m.Parameters {
		flagset.String(k, p.Default, p.Description)
	}
	flagset.Parse(endpointFs.Args()[2:])
	m.call(api)
}

func findMethod(method string, api API) *Method {
	parts := strings.Split(method, ".")
	var ms map[string]Method
	rs := api.Resources
	for i := 0; i < len(parts)-1; i++ {
		r := rs[parts[i]]
		if &r == nil {
			log.Fatal("Could not find requested method ", method)
		}
		rs = r.Resources
		ms = r.Methods
	}
	lp := parts[len(parts)-1]
	m := ms[lp]
	if &m == nil {
		log.Fatal("Could not find requested method ", method)
	}
	return &m
}

func getPreferredVersion(apiName string) (string, error) {
	var d struct {
		Items []struct {
			Version string
		}
	}
	err := getAndParse(fmt.Sprintf("discovery/v1/apis?preferred=true&name=%s&fields=items/version", apiName), &d)
	if err != nil {
		return "", err
	}
	if d.Items == nil {
		log.Fatal("Could not load API ", apiName)
	}
	return d.Items[0].Version, nil
}

// loadAPI takes a string like "apiname" or "apiname:v4" and loads the API from Discovery
func loadAPI(s string) (*API, error) {
	parts := strings.SplitN(s, ":", 2)
	apiName := parts[0]
	var v string
	if len(parts) == 2 {
		v = parts[1]
	} else {
		// Look up preferred version in Directory
		var err error
		v, err = getPreferredVersion(apiName)
		if err != nil {
			log.Fatal(err)
		}
	}

	var a API
	err := getAndParse(fmt.Sprintf("discovery/v1/apis/%s/%s/rest", apiName, v), &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func getAndParse(path string, v interface{}) error {
	url := *flagEndpoint + path

	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	err = json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		return err
	}
	return nil
}

type API struct {
	BaseURL, Name, Title, Description, DocumentationLink string
	Resources                                            map[string]Resource
	Methods                                              map[string]Method
	Parameters                                           map[string]Parameter
}

type Resource struct {
	Resources map[string]Resource
	Methods   map[string]Method
}

type Method struct {
	ID, Path, HttpMethod, Description string
	Parameters                        map[string]Parameter
	Scopes                            []string
}

func (m Method) call(api *API) {
	if m.Scopes != nil {
		scope := strings.Join(m.Scopes, " ")
		if *flagPem != "" && *flagSecrets != "" {
			tok, err := accessTokenFromPemFile(scope, *flagPem, *flagSecrets)
			if err != nil {
				log.Fatal(err)
			}
			log.Print(tok)
		} else {
			log.Fatal("This method requires access to API scopes: ", scope)
		}
	}

	url := api.BaseURL + m.Path
	for k, p := range m.Parameters {
		url = p.process(k, url)
	}
	for k, p := range api.Parameters {
		url = p.process(k, url)
	}

	var body io.Reader
	if *flagStdin {
		// If user passes the --in flag, use stdin as the request body
		body = os.Stdin
	} else if *flagInFile != "" {
		// If user passes --inFile flag, open that file and use its content as request body
		var err error
		body, err = os.Open(*flagInFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	r, err := http.NewRequest(m.HttpMethod, url, body)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	io.Copy(os.Stderr, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		os.Exit(1)
	}
}

func accessTokenFromPemFile(scope, pemPath, secretsPath string) (string, error) {
	pemFile, err := os.Open(pemPath)
	if err != nil {
		return "", err
	}
	defer pemFile.Close()
	keyBytes, err := ioutil.ReadAll(pemFile)
	if err != nil {
		return "", err
	}
	pb, _ := pem.Decode(keyBytes)
	if len(pb.Bytes) == 0 {
		return "", errors.New("No PEM data found")
	}

	secretsFile, err := os.Open(secretsPath)
	if err != nil {
		return "", err
	}
	defer secretsFile.Close()
	secretsBytes, err := ioutil.ReadAll(secretsFile)
	if err != nil {
		return "", err
	}
	var config struct {
		Web struct {
			ClientEmail string `json:"client_email"`
			TokenURI    string `json:"token_uri"`
		}
	}
	err = json.Unmarshal(secretsBytes, &config)
	if err != nil {
		return "", err
	}

	t := jwt.NewToken(config.Web.ClientEmail, scope, pb.Bytes)
	t.ClaimSet.Aud = config.Web.TokenURI
	tok, err := t.Assert(&http.Client{})
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

type Parameter struct {
	Type, Description, Location, Default string
	Required                             bool
}

func (p Parameter) process(k string, url string) string {
	f := flagset.Lookup(k)
	if f == nil {
		return url
	}
	v := f.Value.String()
	if v == "" {
		return url
	}
	if p.Location == "path" {
		if p.Required && v == "" {
			log.Print("Missing required parameter ", k)
		}
		t := fmt.Sprintf("{%s}", k)
		return strings.Replace(url, t, v, -1)
	} else if p.Location == "query" {
		delim := "&"
		if !strings.Contains(url, "?") {
			delim = "?"
		}
		return url + fmt.Sprintf("%s%s=%s", delim, k, v)
	}
	return url
}
