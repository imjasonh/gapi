// Usage: (uh, eventually, once all of this is implemented...)
// $ googlecl help  # prints help
// $ googlecl list  # lists all available APIs
// $ googlecl describe calendar                          # describes all methods available in Calendar API
// $ googlecl describe calendar calendars.get            # describes one method in Calendar API
//
// $ googlecl calendar calendars.get --calendarId=12345  # prints JSON API response
//
// $ cat someEvent.json | googlecl calendar events.insert --calendarId=12345 --in  # inserts an event
// $ googlecl calendar events.insert --calendarId=12345 --inFile=someEvent.json    # equivalent to above
//
// TODO: Handle auth somehow.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var cmds = map[string]func(){
	"help":     func() { log.Fatal("TODO: implement help command") },
	"list":     func() { log.Fatal("TODO: implement list command") },
	"describe": func() { log.Fatal("TODO: implement describe command") },
}

func parseArgs(args []string) map[string]string {
	m := make(map[string]string)
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			a = a[2:]
		} else if strings.HasPrefix(a, "-") {
			a = a[1:]
		} else {
			log.Fatalf("Invalid flag format %s", a)
		}

		if !strings.Contains(a, "=") {
			m[a] = "true"
		} else {
			parts := strings.SplitN(a, "=", 2)
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func main() {
	if len(os.Args) == 1 {
		cmds["help"]()
		return
	}

	cmd := os.Args[1]
	if cmd == "" {
		log.Fatal("Must specify command or API name")
	}
	if cmdFn, found := cmds[cmd]; found {
		cmdFn()
		return
	}

	method := os.Args[2]
	if method == "" {
		log.Fatal("Must specify API method to call")
	}

	apiName := cmd
	fs := parseArgs(os.Args[3:])
	v := flagValue(fs, "v")
	if v == "" {
		// Look up preferred version in Directory
		var err error
		v, err = getPreferredVersion(apiName)
		if err != nil {
			log.Fatal(err)
		}
	}
	api, err := loadApi(apiName, v)
	if err != nil {
		log.Fatal(err)
	}
	if api == nil || (len(api.Resources) == 0 && len(api.Methods) == 0) {
		log.Fatalf("Couldn't load API %s %s", apiName, v)
	}

	m := findMethod(method, *api)
	if m == nil {
		log.Fatalf("Can't find requested method %s", method)
	}

	m.call(fs, apiName, v)
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

func flagValue(fs map[string]string, k string) string {
	v, found := fs[k]
	if !found {
		return ""
	}
	return v
}

func getPreferredVersion(api string) (string, error) {
	var d struct {
		Items []struct {
			Version string
		}
	}
	err := getAndParse(fmt.Sprintf("https://www.googleapis.com/discovery/v1/apis?preferred=true&name=%s&fields=items/version", api), &d)
	if err != nil {
		return "", err
	}
	return d.Items[0].Version, nil
}

func loadApi(api, version string) (*Api, error) {
	var a Api
	err := getAndParse(fmt.Sprintf("https://www.googleapis.com/discovery/v1/apis/%s/%s/rest", api, version), &a)
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
	Resources map[string]Resource
	Methods   map[string]Method
}

type Resource struct {
	Resources map[string]Resource
	Methods   map[string]Method
}

type Method struct {
	Id, Path, HttpMethod string
	Parameters           map[string]Parameter
}

func (m Method) call(fs map[string]string, apiName, version string) {
	url := fmt.Sprintf("https://www.googleapis.com/%s/%s/%s", apiName, version, m.Path)
	for k, p := range m.Parameters {
		v := flagValue(fs, k)
		if p.Location == "path" {
			t := fmt.Sprintf("{%s}", k)
			if p.Required && v == "" {
				log.Printf("Missing required parameter %s", k)
			}
			url = strings.Replace(url, t, v, -1)
		} else if p.Location == "query" {
			if !strings.Contains(url, "?") {
				url += "?"
			}
			url += fmt.Sprintf("&%s=%s", k, v)
		}
	}

	var body io.Reader
	if v, found := fs["in"]; found && v == "true" {
		// If user passes the --in flag, use stdin as the request body
		body = os.Stdin
	} else if v, found := fs["inFile"]; found {
		// If user passes --inFile flag, open that file and use its content as request body
		var err error
		body, err = os.Open(v)
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

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(os.Stderr, resp.Body)
		os.Exit(1)
	} else {
		io.Copy(os.Stdout, resp.Body)
	}
}

type Parameter struct {
	Type, Description, Location string
	Required                    bool
}
