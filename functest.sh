set -e
set -x

go build googlecl.go

# Test help commands
./googlecl | grep "Makes requests to Google APIs"
./googlecl help | grep "Makes requests to Google APIs"
./googlecl list | grep "calendar v3"
./googlecl help calendar | grep "Calendar API"
./googlecl help prediction:v1.4 | grep "Prediction API"
./googlecl help calendar events.list | grep "events.list Returns events on the specified calendar."

# Test help commands for Cloud Endpoints APIs
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/ help
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/ list
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/ help greeting
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/ help greeting greets.list

# Test calling an API
./googlecl urlshortener url.get --shortUrl=http://goo.gl/fUhtIm
./googlecl urlshortener:v1 url.get --shortUrl=http://goo.gl/fUhtIm
./googlecl urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl
./googlecl --endpoint=https://www.googleapis.com/ urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl

# Test calling a Cloud Endpoints API
./googlecl --endpoint=https://go-endpoints.appspot.com/_ah/api/ greeting greets.list
rm googlecl
