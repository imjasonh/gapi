set -e
set -x

go build gapi.go auth.go

# Test help commands
./gapi | grep "Makes requests to Google APIs"
./gapi help | grep "Makes requests to Google APIs"
./gapi list | grep "calendar v3"
./gapi help calendar | grep "Calendar API"
./gapi help prediction:v1.4 | grep "Prediction API"
./gapi help calendar events.list | grep "events.list Returns events on the specified calendar."

# Test help commands for Cloud Endpoints APIs
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/ help
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/ list
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/ help greeting
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/ help greeting greets.list

# Test calling an API
./gapi urlshortener url.get --shortUrl=http://goo.gl/fUhtIm
./gapi urlshortener:v1 url.get --shortUrl=http://goo.gl/fUhtIm
./gapi urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl
./gapi --endpoint=https://www.googleapis.com/ urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl

# Test calling a Cloud Endpoints API
./gapi --endpoint=https://go-endpoints.appspot.com/_ah/api/ greeting greets.list
rm gapi
