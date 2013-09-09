set -e
set -x

go build googlecl.go
./googlecl | grep "Makes requests to Google APIs"
./googlecl help | grep "Makes requests to Google APIs"
./googlecl help calendar | grep "Calendar API"
./googlecl help calendar events.list | grep "events.list Returns events on the specified calendar."
./googlecl list | grep "calendar v3"
./googlecl urlshortener url.get --shortUrl=http://goo.gl/fUhtIm
./googlecl urlshortener:v1 url.get --shortUrl=http://goo.gl/fUhtIm
./googlecl urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl
./googlecl greeting greets.list --meta.endpoint=https://go-endpoints.appspot.com/_ah/api/
rm googlecl