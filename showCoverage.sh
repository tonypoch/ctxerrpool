echo "check your web browser"
find cmd/profiles/ -type f -exec go tool cover -html={} \;
