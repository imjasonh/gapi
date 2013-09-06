googlecl
========

Go rewrite of Google API CLI tool

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

Insert a resource
```
$ echo "{\"longUrl\": \"http://github.com/imjasonh/googlecl\"}" | go run googlecl.go urlshortener url.insert --in
{
 "kind": "urlshortener#url",
 "id": "http://goo.gl/0BErIk",
 "longUrl": "http://github.com/imjasonh/googlecl"
}
```

Users can use a tool like [jq][1] to slice and dice responses and create request body data.

[1]: http://stedolan.github.io/jq/
