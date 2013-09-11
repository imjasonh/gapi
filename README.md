googlecl.go: A Go rewrite of Google API CLI tool
================================================
(original Python googlecl script is at https://code.google.com/p/googlecl)

Getting Help
------------

List available APIs
```
$ go run googlecl.go list
adexchangebuyer v1 - Lets you manage your Ad Exchange Buyer account.
adexchangebuyer v1.1 - Lets you manage your Ad Exchange Buyer account.
...
```

Get information about a specific API
```
$ go run googlecl.go help calendar
Calendar API Lets you manipulate events and other calendar data.
More information: https://developers.google.com/google-apps/calendar/firstapp
Methods:
...
```

Get information about a specific method
```
$ go run googlecl.go help calendar events.list
events.list Returns events on the specified calendar.
Parameters:
...
```

Calling API Methods
-------------------

API requests print JSON to stdout. Users can use a tool like [jq][1] to slice and dice responses.

Get a resource
```
$ go run googlecl.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm
{
 "kind": "urlshortener#url",
 "id": "http://goo.gl/fUhtIm",
 "longUrl": "https://github.com/ImJasonH/googlecl/",
 "status": "OK"
}
```

Get certain fields of a resource
```
$ go run googlecl.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl
{
 "longUrl": "https://github.com/ImJasonH/googlecl/",
}
```

Cloud Endpoints APIs
--------------------

To use [Cloud Endpoints APIs][2], pass the `--endpoint=` flag _before_ the command or method to invoke, like so:

```
$ go run googlecl.go --endpoint=https://go-endpoints.appspot.com/_ah/api/ list
Available methods:
...
```

```
go run googlecl.go --endpoint=https://go-endpoints.appspot.com/_ah/api/ greeting greets.list
{
 "items": [
    ...
  ]
}
```

[1]: http://stedolan.github.io/jq/
[2]: https://developers.google.com/appengine/docs/java/endpoints/
