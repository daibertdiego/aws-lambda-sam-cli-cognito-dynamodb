require (
	github.com/aws/aws-lambda-go v1.23.0
	github.com/aws/aws-sdk-go v1.43.3
	github.com/aws/aws-sdk-go-v2/config v1.13.1
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.6.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.13.0
	github.com/go-playground/validator/v10 v10.10.0
	github.com/gookit/validate v1.2.11
	github.com/rs/xid v1.3.0
)

replace gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.8

module hello-world

go 1.16
