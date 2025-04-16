# AWS Lambda Last Invocation Lister
CLI tool to list all of your AWS Lambda Functions and their last invocation time and output the result in a CSV file

## Getting started
You can download the compiled binaries from the release page. By default, it will use your "default" profile from your AWS CLI configuration
```shell
alli-lister
```

If you want to use different profile, you can use `--aws-profile` argument
```shell
alli-lister --aws-profile <your-profile-name>
```

To run it in debug mode for troubleshooting, set `--debug=true`
```shell
alli-lister -debug=true
```

## Directly running the source code
You can also run the source code directly if you have Go installed
```shell
go run .

# or if you have Make installed
make run
```

If you want to use different profile, you can use `--aws-profile` argument
```shell
go run . --aws-profile <your-profile-name>
```

To run it in debug mode for troubleshooting, set `--debug=true`
```shell
go run . -debug=true

# or if you have Make installed
make debug
```
