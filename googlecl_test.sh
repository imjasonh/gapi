set -e
set -x

go run googlecl.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm | grep "http://goo.gl/fUhtIm"
go run googlecl.go urlshortener url.get --shortUrl=http://goo.gl/fUhtIm --fields=longUrl | grep "/ImJasonH/googlecl/"
