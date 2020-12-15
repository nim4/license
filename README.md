# license
Dependencies license check for Go

# Usage
```
Usage of license-check:
  -files string
        Comma separated list of license file names - case insensitive (default "LICENSE,LICENSE.TXT,LICENSE.MD,COPYING")
  -output string
        Write per dependency license to 'json' file
  -path string
        Path of 'vendor' directory (default "./vendor/")
  -timeout duration
        Max execution time of the license check (default 5m0s)
```

`.license` file example:
```
# Allow only
MIT
Apache-2.0
```
