# ccl, the worst human-friendly configuration language

Don't like YAML? TOML's nested tables make your head hurt? Give me a few hours,
how hard can writing a recursive-descent parser be...

## Examples

    # nginx config or something
    server {
        listen: "0.0.0.0:80"
        listen: "[::0]:80"
        location {
            path: "/"
            return: "301 https://$host$request_uri" # redirect to https
        }
        location {
            path: "/.well-known/acme-challenge/"
            root: "/var/lib/acme/acme-challenge"
            auth_basic: off
            auth_request: off
        }
    }

See [https://pkg.go.dev/roseh.moe/pkg/ccl](https://pkg.go.dev/roseh.moe/pkg/ccl)
for the full language spec.
