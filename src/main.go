package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-playground/validator/v10"
	"github.com/rs/xid"
)

const tableName = "User"

var (
	ErrAuthorizerNotConfigured = errors.New("Authorization not configured")

	ErrUserNotInformed = errors.New("User body required.")

	ErrUserNotCreated = errors.New("User could not be created")

	ErrUserNotFound = errors.New("User not found")

	ErrUserAlreadyExists = errors.New("User already exists with this e-mail")
)

type User struct {
	Id    string `json:"id" dynamodbav:"id"`
	Email string `json:"e-mail" dynamodbav:"email" validate:"required,email"`
	Name  string `json:"name" dynamodbav:"name" validate:"required,min=2,max=200"`
	Age   int16  `json:"age" dynamodbav:"age"`
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)
	log.Printf("RequestID %s", lc.AwsRequestID)
	log.Printf("Authorizer %s", request.RequestContext.Authorizer)

	devEnv, _ := strconv.ParseBool(os.Getenv("AWS_SAM_LOCAL"))

	if !devEnv {
		if nil == request.RequestContext.Authorizer {
			return errorResponse(ErrAuthorizerNotConfigured, http.StatusInternalServerError)
		}
		claims := request.RequestContext.Authorizer["claims"].(map[string]interface{})
		log.Printf("Token claims %s", claims)
		log.Printf("Logged user %s", claims["email"])
		log.Printf("Logged user account %s", claims["custom:Accounts"])
	} else {
		authorizer := make(map[string]interface{})
		authorizer["claims"] = map[string]interface{}{
			"at_hash":          "Q8yOMWKcs3sJGcwsFpWTAg",
			"sub":              "f8b122b0-f272-4715-826b-fbaf3f532a3f",
			"email_verified":   true,
			"iss":              "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_397EAYA3A",
			"cognito:username": "f8b122b0-f272-4715-826b-fbaf3f532a3f",
			"aud":              "7uvk2dvgl7adp285c5t5vibb6a",
			"custom:Accounts":  "ROOT",
			"token_use":        "id",
			"auth_time":        1645590467,
			"exp":              1645594067,
			"iat":              1645590467,
			"jti":              "fdea9783-5853-4a11-8c7c-7f3dc406e7ad",
			"email":            "daibertdiego@gmail.com",
		}

		request.RequestContext.Authorizer = authorizer

		mockedClaims := request.RequestContext.Authorizer["claims"].(map[string]interface{})
		log.Printf("Token claims %s", mockedClaims)
		log.Printf("Logged user %s", mockedClaims["email"])
		log.Printf("Logged user account %s", mockedClaims["custom:Accounts"])
	}

	user := User{}
	uuid := xid.New()
	if err := json.Unmarshal([]byte(request.Body), &user); err != nil {
		log.Println(ErrUserNotCreated)
		return errorResponse(ErrUserNotInformed, http.StatusBadRequest)
	}

	validate := validator.New()
	if err := validate.Struct(user); err != nil {
		log.Println(err)
		return errorResponse(err, http.StatusBadRequest)
	}

	user.Id = uuid.String()

	err := save(user)
	if err != nil {
		return errorResponse(err, http.StatusInternalServerError)
	}

	body, _ := json.Marshal(user)

	return events.APIGatewayProxyResponse{
		Body:       string(body),
		StatusCode: 200,
	}, nil
}

func dynamoDbConnection() *dynamodb.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	dynamoService := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		devEnv, _ := strconv.ParseBool(os.Getenv("AWS_SAM_LOCAL"))
		if devEnv {
			o.EndpointResolver = dynamodb.EndpointResolverFromURL("http://dynamodb:8000")
		}
	})
	return dynamoService
}

func save(item User) error {
	userExists, err := emailAlreadyExists(item.Email)
	if err != nil {
		log.Println(err)
		return err
	}
	if userExists {
		return ErrUserAlreadyExists
	}
	dynamoService := dynamoDbConnection()
	av, err := attributevalue.MarshalMap(item)
	log.Println(av)
	if err != nil {
		log.Println(err)
		errorResponse(ErrUserNotCreated, http.StatusInternalServerError)
	}
	out, err := dynamoService.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Println(err)
		return ErrUserNotCreated
	}
	log.Println(out.Attributes)
	return nil
}

func emailAlreadyExists(email string) (bool, error) {
	dynamoService := dynamoDbConnection()
	paginator := dynamodb.NewScanPaginator(dynamoService, &dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(context.TODO())

		if err != nil {
			log.Println(err)
			return false, ErrUserNotFound
		}
		if out.Count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func errorResponse(err error, statusCode int) (events.APIGatewayProxyResponse, error) {
	resp := events.APIGatewayProxyResponse{
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
	}
	resp.StatusCode = statusCode

	errStr, _ := json.Marshal(err.Error())
	resp.Body = string(errStr)
	return resp, nil
}

func main() {
	lambda.Start(handler)
}
