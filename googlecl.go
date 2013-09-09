// TODO: Handle user auth.
// TODO: Verify service account auth actually works.
// TODO: Cache discovery/directory documents for faster requests.
// TODO: Handle media upload/download.
// TODO: Handle repeated parameters.
// TODO: Support Cloud Endpoints APIs.

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

var cmds = map[string]func(){
	"help": help,
	"list": list,
}

var (
	flagset = flag.NewFlagSet("", flag.ExitOnError)

	flagPem    = flagset.String("meta.pem", "", "Location of .pem file")
	flagIss    = flagset.String("meta.iss", "", "Service account email address")
	flagStdin  = flagset.Bool("meta.in", false, "Accept request body from stdin")
	flagInFile = flagset.String("meta.inFile", "", "File to pass as request body")
)

func help() {
	args := len(os.Args)
	if args < 3 || args > 4 {
		fmt.Println("Makes requests to Google APIs")
		fmt.Println("Usage:")
		fmt.Println("googlecl <api> <method> --param=foo")
		fmt.Println("Flags:")
		flagset.VisitAll(func(f *flag.Flag) {
			fmt.Printf("--%s - %s\n", f.Name, f.Usage)
		})
	} else {
		apiName := os.Args[2]
		api, err := loadApi(apiName)
		if err != nil {
			log.Fatal(err)
		}
		if args == 3 {
			fmt.Println(api.Title, api.Description)
			fmt.Println("More information:", api.DocumentationLink)
			fmt.Println("Methods:")
			for _, m := range api.Methods {
				fmt.Println(m.Id, m.Description)
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
					fmt.Printf("%s - %s\n", m.Id[len(apiName)+1:], m.Description)
				}
				for k, r := range r.Resources {
					l = append(l, pair{k, r})
				}
			}
		} else if args == 4 {
			method := os.Args[3]
			m := findMethod(method, *api)
			fmt.Println(method, m.Description)
			fmt.Println("Parameters:")
			for k, p := range m.Parameters {
				fmt.Printf("--%s (%s) - %s\n", k, p.Type, p.Description)
			}
			for k, p := range api.Parameters {
				fmt.Printf("--%s (%s) - %s\n", k, p.Type, p.Description)
			}
		}
	}
}

func list() {
	var directory struct {
		Items []struct {
			Name, Version, Description string
		}
	}
	getAndParse("https://www.googleapis.com/discovery/v1/apis", &directory)
	fmt.Println("Available methods:")
	for _, i := range directory.Items {
		fmt.Printf("%s %s - %s\n", i.Name, i.Version, i.Description)
	}
}

func main() {
	if len(os.Args) == 1 {
		help()
		return
	}
	cmd := os.Args[1]
	if cmd == "help" {
		help()
		return
	} else if cmd == "list" {
		list()
		return
	}

	method := os.Args[2]
	if method == "" {
		log.Fatal("Must specify API method to call")
	}

	api, err := loadApi(cmd)
	if err != nil {
		log.Fatal(err)
	}
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", cmd)
	}

	m := findMethod(method, *api)
	if m == nil {
		log.Fatal("Can't find requested method ", method)
	}

	for k, p := range api.Parameters {
		flagset.String(k, p.Default, p.Description)
	}
	for k, p := range m.Parameters {
		flagset.String(k, p.Default, p.Description)
	}
	flagset.Parse(os.Args[3:])

	m.call(api)
}

func findMethod(method string, api Api) *Method {
	parts := strings.Split(method, ".")
	var ms map[string]Method
	rs := api.Resources
	for i := 0; i < len(parts)-1; i++ {
		r, found := rs[parts[i]]
		if !found {
			return nil
		}
		rs = r.Resources
		ms = r.Methods
	}
	lp := parts[len(parts)-1:][0]
	m, found := ms[lp]
	if !found {
		return nil
	}
	return &m
}

func flagValue(k string) string {
	f := flagset.Lookup(k)
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func getPreferredVersion(apiName string) (string, error) {
	var d struct {
		Items []struct {
			Version string
		}
	}
	err := getAndParse(fmt.Sprintf("https://www.googleapis.com/discovery/v1/apis?preferred=true&name=%s&fields=items/version", apiName), &d)
	if err != nil {
		return "", err
	}
	if d.Items == nil {
		log.Fatal("Could not load API ", apiName)
	}
	return d.Items[0].Version, nil
}

// loadApi takes a string like "apiname" or "apiname:v4" and loads the API from Discovery
func loadApi(s string) (*Api, error) {
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

	var a Api
	err := getAndParse(fmt.Sprintf("https://www.googleapis.com/discovery/v1/apis/%s/%s/rest", apiName, v), &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func getAndParse(url string, v interface{}) error {
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

type Api struct {
	BaseUrl, Title, Description, DocumentationLink string
	Resources                                      map[string]Resource
	Methods                                        map[string]Method
	Parameters                                     map[string]Parameter
}

type Resource struct {
	Resources map[string]Resource
	Methods   map[string]Method
}

type Method struct {
	Id, Path, HttpMethod, Description string
	Parameters                        map[string]Parameter
	Scopes                            []string
}

func (m Method) call(api *Api) {
	if m.Scopes != nil {
		scope := strings.Join(m.Scopes, " ")
		if flagPem != nil && flagIss != nil {
			// TODO: Get iss from client_secrets.json
			tok, err := accessTokenFromPemFile(*flagIss, scope, *flagPem)
			if err != nil {
				log.Fatal(err)
			}
			log.Print(tok)
		} else {
			log.Fatal("This method requires access to API scopes: ", scope)
		}
	}

	url := api.BaseUrl + m.Path
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

func accessTokenFromPemFile(iss, scope, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	pb, _ := pem.Decode(b)
	if len(pb.Bytes) == 0 {
		return "", errors.New("No PEM data found")
	}

	t := jwt.NewToken(iss, scope, pb.Bytes)
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
	v := flagValue(k)
	if v == "" {
		return url
	}
	if p.Location == "path" {
		t := fmt.Sprintf("{%s}", k)
		if p.Required && v == "" {
			log.Print("Missing required parameter ", k)
		}
		return strings.Replace(url, t, v, -1)
	} else if p.Location == "query" {
		if !strings.Contains(url, "?") {
			url += "?"
		}
		return url + fmt.Sprintf("&%s=%s", k, v)
	}
	return url
}
