echo "check your web browser"
find cmd/profiles/ -type f -name "*.out" -exec go tool cover -html={} \;
