gapi: A command-line interface to Google APIs
================================================
(NOTE: This is not an official Google product)

Installation
------------
  * [Install Go][3]
  * `go get code.google.com/p/goauth2/oauth code.google.com/p/goauth2/oauth/jwt`
  * `go run gapi.go ...`, or `go build gapi.go` then run `gapi ...`

Learning About APIs
-------------------

List available APIs
```
$ go run gapi.go list
adexchangebuyer v1 - Lets you manage your Ad Exchange Buyer account.
adexchangebuyer v1.1 - Lets you manage your Ad Exchange Buyer account.
...
```

Get information about a specific API
```
$ go run gapi.go help calendar
Calendar API Lets you manipulate events and other calendar data.
More information: https://developers.google.com/google-apps/calendar/firstapp
Methods:
...
```

Get information about a specific method
```
$ go run gapi.go help calendar events.list
events.list Returns events on the specified calendar.
Parameters:
...
```

Calling API Methods
-------------------

API requests print JSON to stdout. Users can use a tool like [jq][1] to slice and dice responses.

Get a resource
```
$ go run gapi.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm
{
 "kind": "urlshortener#url",
 "id": "http://goo.gl/fUhtIm",
 "longUrl": "https://github.com/ImJasonH/gapi/",
 "status": "OK"
}
```

Get certain fields of a resource
```
$ go run gapi.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl
{
 "longUrl": "https://github.com/ImJasonH/gapi/",
}
```

Insert a new resource
```
$ go run gapi.go urlshortener url.insert --meta.pem=example.pem --meta.secrets=client_secrets.json --meta.inFile=url.json
{
 "kind": "urlshortener#url",
 "id": "http://goo.gl/POIxRL",
 "longUrl": "https://github.com/ImJasonH/gapi"
}
```
or
```
$ echo '{"longUrl":"https://github.com/ImJasonH/gapi"}' | go run gapi.go urlshortener url.insert --meta.pem=example.pem --meta.secrets=client_secrets.json --meta.in
```
(Make sure to pass the --meta.in flag to tell gapi to read from stdin)

or, for simple request bodies
```
$ go run gapi.go urlshortener url.insert --meta.pem=example.pem --meta.secrets=client_secrets.json --res.longUrl=https://github.com/ImJasonH/gapi
```
(This syntax is currently only supported for top-level request fields)

Cloud Endpoints APIs
--------------------

To use [Cloud Endpoints APIs][2], pass the `--endpoint=` flag _before_ the command or method to invoke, like so:

```
$ go run gapi.go --endpoint=https://go-endpoints.appspot.com/_ah/api/ list
Available methods:
...
```

```
go run gapi.go --endpoint=https://go-endpoints.appspot.com/_ah/api/ greeting greets.list
{
 "items": [
    ...
  ]
}
```

[1]: http://stedolan.github.io/jq/
[2]: https://developers.google.com/appengine/docs/java/endpoints/
[3]: http://golang.org/doc/install
