# AWS Lambda Last Invocation Lister
CLI tool to list all of your AWS Lambda Functions and their last invocation time and output the result in a CSV file

## Getting started
By default, it will use your "default" profile in your AWS CLI configuration
```shell
go run .
```

If you want to use different profile, you can use `--aws-profile` argument
```shell
go run . --aws-profile <your-profile-name>
```

To run it in debug mode for troubleshooting, set `--debug=true`
```shell
go run . -debug=true
```
