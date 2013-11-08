// TODO: Handle user auth.
// TODO: Cache discovery/directory documents for faster requests.
// TODO: Handle media upload/download.
// TODO: Handle repeated parameters.

package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/goauth2/oauth/jwt"
)

var (
	// Flags that get parsed before the command, necessary for loading Cloud Endpoints APIs
	// e.g., "gapi --endpoint=foo help myapi" parses the endpoint flag before loading the API
	endpointFs   = flag.NewFlagSet("endpoint", flag.ExitOnError)
	flagEndpoint = endpointFs.String("endpoint", "https://www.googleapis.com/", "Cloud Endpoints URL, e.g., https://my-app-id.appspot.com/_ah/api/")

	// Flags that get parsed after the command, common to all APIs
	fs          = flag.NewFlagSet("gapi", flag.ExitOnError)
	flagPem     = fs.String("meta.pem", "", "Location of .pem file")
	flagSecrets = fs.String("meta.secrets", "", "Location of client_secrets.json")
	flagInFile  = fs.String("meta.inFile", "", "File to pass as request body")
	flagStdin   = fs.Bool("meta.in", false, "Whether to use stdin as the request body")
	flagToken   = fs.String("meta.token", "", "OAuth 2.0 access token to use")
	oauthConfig = &oauth.Config{
		ClientId:       "68444827642.apps.googleusercontent.com",
		ClientSecret:   "K62E0K7ldOYkwUos3GrNkzU4",
		RedirectURL:    "urn:ietf:wg:oauth:2.0:oob",
		AccessType:     "offline",
		ApprovalPrompt: "force",
		Scope:          "",
		AuthURL:        "https://accounts.google.com/o/oauth2/auth",
		TokenURL:       "https://accounts.google.com/o/oauth2/token",
	}
)

func maybeFatal(msg string, err error) {
	if err != nil {
		log.Fatal(msg, " ", err)
	}
}

func simpleHelp() {
	fmt.Println("Makes requests to Google APIs")
	fmt.Println("Usage:")
	fmt.Println("  gapi <api> <method> --param=foo")
}

func help() {
	args := endpointFs.Args()
	nargs := len(args)
	if nargs == 0 || (nargs == 1 && args[0] == "help") {
		simpleHelp()
		return
	}
	apiName := args[1]
	api := loadAPI(apiName)
	if nargs == 2 {
		// gapi help <api>
		fmt.Println(api.Title, api.Description)
		fmt.Println("More information:", api.DocumentationLink)
		fmt.Println("Methods:")
		for _, m := range api.Methods {
			fmt.Println("-", m.ID, m.Description)
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
				fmt.Printf("- %s: %s\n", m.ID[len(api.Name)+1:], m.Description)
			}
			for k, r := range r.Resources {
				l = append(l, pair{k, r})
			}
		}
	} else {
		// gapi help <api> <method>
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
		s := api.Schemas[m.RequestSchema.Ref]
		// TODO: Support deep nested schemas, and use actual flags to get these strings to avoid duplication
		for k, p := range s.Properties {
			fmt.Printf("  --res.%s (%s) - %s\n", k, p.Type, p.Description)
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
		fmt.Printf("- %s %s - %s\n", i.Name, i.Version, i.Description)
	}
}

func authStart() {
	args := endpointFs.Args()
	if len(args) != 3 {
		fmt.Println("Invalid arguments, must provide API and method, e.g.:")
		fmt.Println("  gapi auth.start <api> <method>")
		return
	}
	api := loadAPI(args[1])
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", args[1])
	}
	m := findMethod(args[2], *api)
	if m.Scopes == nil {
		log.Fatal("Method doesn't require auth")
	}
	oauthConfig.Scope = strings.Join(m.Scopes, " ")
	fmt.Println("Open a browser and visit the following URL:")
	fmt.Println(oauthConfig.AuthCodeURL(""))
	fmt.Println("Then run the following command with the resulting auth code:")
	fmt.Println("gapi auth.finish <code>")
}

func authFinish() {
	args := endpointFs.Args()
	if len(args) != 2 {
		fmt.Println("Invalid arguments, must provide code, e.g.:")
		fmt.Println("  gapi auth.finish <code>")
		return
	}
	code := args[1]
	t := &oauth.Transport{Config: oauthConfig}
	tok, err := t.Exchange(code)
	if err != nil {
		log.Fatal(err)
	}

	toks, err := loadTokens()
	if err != nil {
		log.Fatal(err)
	}
	inf, err := getTokenInfo(tok.AccessToken)
	if err != nil {
		log.Fatal("getTokenInfo", err)
	}
	toks.Tok[inf.Scope] = token{
		AccessToken: tok.AccessToken,
		RefreshToken: tok.RefreshToken,
	}
	if err = toks.save(); err != nil {
		log.Fatal("save", err)
	}
	fmt.Println("Token saved")
}

func authPrint() {
	args := endpointFs.Args()
	if len(args) != 3 {
		fmt.Println("Invalid arguments, must provide API and method, e.g.:")
		fmt.Println("  gapi auth.print <api> <method>")
		return
	}
	api := loadAPI(args[1])
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", args[1])
	}
	m := findMethod(args[2], *api)
	toks, err := loadTokens()
	if err != nil {
		log.Fatal("Could not determine token")
	}
	if tok, found := toks.Tok[strings.Join(m.Scopes, " ")]; found {
		// TODO: If necessary, refresh, store, and print the new token.
		fmt.Println(tok.AccessToken)
	} else {
		fmt.Println("No token found. Run the following command to store a token:")
		fmt.Println("gapi auth.start", api.Name, m.ID[len(api.Name)+1:])
	}
		
}

func main() {
	endpointFs.Parse(os.Args[1:])
	if len(endpointFs.Args()) == 0 {
		simpleHelp()
		return
	}

	cmd := endpointFs.Args()[0]

	switch cmd {
	case "help":
		help()
		return
	case "list":
		list()
		return
	case "auth.start":
		authStart()
		return
	case "auth.finish":
		authFinish()
		return
	case "auth.print":
		authPrint()
		return
	}

	api := loadAPI(cmd)
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatal("Couldn't load API ", cmd)
	}

	if len(endpointFs.Args()) == 1 {
		fmt.Println("Must specify a method to call")
		fmt.Printf("Run \"gapi help %s\" to see a list of available methods\n", cmd)
		return
	}
	method := endpointFs.Args()[1]
	m := findMethod(method, *api)
	for k, p := range api.Parameters {
		fs.String(k, p.Default, p.Description)
	}
	for k, p := range m.Parameters {
		fs.String(k, p.Default, p.Description)
	}

	// TODO: Support deep nested schemas
	s := api.Schemas[m.RequestSchema.Ref]
	for pk, p := range s.Properties {
		fs.String("res."+pk, "", "Request body: "+p.Description)
	}

	fs.Parse(endpointFs.Args()[2:])
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

func getPreferredVersion(apiName string) string {
	var d struct {
		Items []struct {
			Version string
		}
	}
	getAndParse(fmt.Sprintf("discovery/v1/apis?preferred=true&name=%s&fields=items/version", apiName), &d)
	if d.Items == nil {
		log.Fatal("Could not load API ", apiName)
	}
	return d.Items[0].Version
}

// loadAPI takes a string like "apiname" or "apiname:v4" and loads the API from Discovery
func loadAPI(s string) *API {
	parts := strings.SplitN(s, ":", 2)
	apiName := parts[0]
	var v string
	if len(parts) == 2 {
		v = parts[1]
	} else {
		// Look up preferred version in Directory
		v = getPreferredVersion(apiName)
	}

	var a API
	getAndParse(fmt.Sprintf("discovery/v1/apis/%s/%s/rest", apiName, v), &a)
	return &a
}

func getAndParse(path string, v interface{}) {
	url := *flagEndpoint + path

	r, err := http.Get(url)
	maybeFatal("error getting "+url, err)
	defer r.Body.Close()
	err = json.NewDecoder(r.Body).Decode(v)
	maybeFatal("error decoding JSON", err)
}

type API struct {
	BaseURL, Name, Title, Description, DocumentationLink string
	Resources                                            map[string]Resource
	Methods                                              map[string]Method
	Parameters                                           map[string]Parameter
	Schemas                                              map[string]Schema
}

type Resource struct {
	Resources map[string]Resource
	Methods   map[string]Method
}

type Method struct {
	ID, Path, HttpMethod, Description string
	Parameters                        map[string]Parameter
	Scopes                            []string
	RequestSchema                     struct {
		Ref string `json:"$ref"`
	} `json:"request"`
}

func (m Method) call(api *API) {
	url := api.BaseURL + m.Path

	for k, p := range m.Parameters {
		api.Parameters[k] = p
	}
	for k, p := range api.Parameters {
		f := fs.Lookup(k)
		if f == nil || f.Value.String() == "" {
			continue
		}
		v := f.Value.String()
		if p.Location == "path" {
			if p.Required && v == "" {
				log.Fatal("Missing required parameter", k)
			}
			t := fmt.Sprintf("{%s}", k)
			strings.Replace(url, t, v, -1)
		} else if p.Location == "query" {
			delim := "&"
			if !strings.Contains(url, "?") {
				delim = "?"
			}
			url += fmt.Sprintf("%s%s=%s", delim, k, v)
		}
	}

	r, err := http.NewRequest(m.HttpMethod, url, nil)
	maybeFatal("error creating request:", err)

	// Add request body
	r.Header.Set("Content-Type", "application/json")
	if *flagInFile != "" {
		r.Body, r.ContentLength = bodyFromFile()
	} else if *flagStdin {
		r.Body, r.ContentLength = bodyFromStdin()
	} else {
		r.Body, r.ContentLength = bodyFromFlags(*api, m)
	}

	// Add auth header
	if *flagToken != "" {
		r.Header.Set("Authorization", "Bearer "+*flagToken)
	} else if m.Scopes != nil {
		scope := strings.Join(m.Scopes, " ")
		if *flagPem != "" && *flagSecrets != "" {
			r.Header.Set("Authorization", "Bearer "+accessTokenFromPemFile(scope))
		} else {
			toks, err := loadTokens()
			if err != nil {
				log.Fatalf("loadTokens %+v", err)
			}
			if tok, found := toks.Tok[scope]; found {
				// TODO: Check expiration and refresh if necessary
				r.Header.Set("Authorization", "Bearer "+tok.AccessToken)
			} else {
				fmt.Println("This method requires access to protected resources")
				fmt.Println("Run the following command to get an auth token:")
				fmt.Println("gapi auth.start", api.Name, m.ID[len(api.Name)+1:])
				return
			}
		}
	}

	client := &http.Client{}
	resp, err := client.Do(r)
	maybeFatal("error making request:", err)
	defer resp.Body.Close()

	io.Copy(os.Stderr, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		os.Exit(1)
	}
}

func bodyFromStdin() (io.ReadCloser, int64) {
	b, err := ioutil.ReadAll(os.Stdin)
	maybeFatal("error reading from stdin:", err)
	return ioutil.NopCloser(bytes.NewReader(b)), int64(len(b))
}

func bodyFromFile() (io.ReadCloser, int64) {
	b, err := ioutil.ReadFile(*flagInFile)
	maybeFatal("error opening file:", err)
	return ioutil.NopCloser(bytes.NewReader(b)), int64(len(b))
}

func bodyFromFlags(api API, m Method) (io.ReadCloser, int64) {
	s := api.Schemas[m.RequestSchema.Ref]
	request := make(map[string]interface{})
	for k, p := range s.Properties {
		f := fs.Lookup("res." + k)
		if f == nil || f.Value.String() == "" {
			continue
		}
		v := f.Value.String()
		request[k] = toType(p.Type, v)
	}
	if len(request) != 0 {
		body, err := json.Marshal(&request)
		maybeFatal("error marshalling JSON:", err)
		return ioutil.NopCloser(bytes.NewReader(body)), int64(len(body))
	}
	return nil, 0
}

func toType(t, v string) interface{} {
	if t == "string" {
		return v
	} else if t == "boolean" {
		return v == "true"
	} else if t == "integer" {
		i, err := strconv.ParseInt(v, 10, 64)
		maybeFatal("error converting "+v+": ", err)
		return int64(i)
	} else if t == "number" {
		f, err := strconv.ParseFloat(v, 64)
		maybeFatal("error convert "+v+": ", err)
		return float64(f)
	} else {
		log.Fatal(fmt.Sprintf("unable to convert %s to type %s", v, t))
	}
	return "unreachable"
}

func accessTokenFromPemFile(scope string) string {
	secretBytes, err := ioutil.ReadFile(*flagSecrets)
	maybeFatal("error reading secrets file:", err)
	var config struct {
		Web struct {
			ClientEmail string `json:"client_email"`
			TokenURI    string `json:"token_uri"`
		}
	}
	err = json.Unmarshal(secretBytes, &config)
	maybeFatal("error unmarshalling secrets:", err)

	keyBytes, err := ioutil.ReadFile(*flagPem)
	maybeFatal("error reading private key file:", err)

	// Craft the ClaimSet and JWT token.
	t := jwt.NewToken(config.Web.ClientEmail, scope, keyBytes)
	t.ClaimSet.Aud = config.Web.TokenURI

	// We need to provide a client.
	c := &http.Client{}

	// Get the access token.
	o, err := t.Assert(c)
	maybeFatal("assertion error:", err)

	return o.AccessToken
}

type Parameter struct {
	Type, Description, Location, Default string
	Required                             bool
}

type Schema struct {
	Type       string
	Properties map[string]Property
}

type Property struct {
	Ref               string `json:"$ref"`
	Type, Description string
	Items             struct {
		Ref string `json:"$ref"`
	}
}

type tokenInfo struct {
	Scope      string
	ExpiresIn  int    `json:"expires_in"`
	AccessType string `json:"access_type"`
}

func (inf tokenInfo) expired() bool {
	return inf.ExpiresIn < 0
}

func getTokenInfo(tok string) (*tokenInfo, error) {
	r, err := http.Post("https://www.googleapis.com/oauth2/v2/tokeninfo?access_token="+tok, "", nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	var info tokenInfo
	err = json.NewDecoder(r.Body).Decode(&info)
	return &info, err
}

func revokeToken(tok string) error {
	_, err := http.Get("https://accounts.google.com/o/oauth2/revoke?token=" + tok)
	return err
}

type tokens struct {
	Tok map[string]token
}
type token struct {
	AccessToken, RefreshToken string
}

const tokensFile = "~tokens.gob"

func loadTokens() (*tokens, error) {
	fi, err := os.Stat(tokensFile)
	if err == os.ErrNotExist || err == io.EOF {
		return &tokens{make(map[string]token)}, nil
	} else if err != nil {
		return &tokens{make(map[string]token)}, nil
	}
	if fi.Size() == 0 {
		return &tokens{make(map[string]token)}, nil
	}

	f, err := os.Open(tokensFile)
	if err != nil && err != io.EOF {
		return &tokens{make(map[string]token)}, nil
	}
	defer f.Close()
	var t tokens
	_ = gob.NewDecoder(f).Decode(&t)
	return &t, nil
}

func (t *tokens) save() error {
	f, err := os.Create(tokensFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(t)
}
