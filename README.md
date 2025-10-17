# Test Drive

A script language to run commands and match results using [CUE](https://cuelang.org).

```
# Fetch Google
HTTP GET https://google.com

MATCH ^END
status: code: 200
headers: [string]: [...string]
body: =~"Google"
END

# Fetch Pet Store OpenAPI Sample
HTTP GET https://petstore3.swagger.io/api/v3/openapi.json

MATCH ^END
body: close({
    openapi: =~"^3\\..+",
    info: _
    externalDocs: _
    servers: [...]
    tags: [...]
    paths: _
    components: _
})
END
```
