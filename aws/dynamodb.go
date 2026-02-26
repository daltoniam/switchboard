package aws

import (
	"context"
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func dynamoListTables(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &dynamodb.ListTablesInput{}
	if v := argInt32(args, "limit"); v > 0 {
		input.Limit = aws.Int32(v)
	}
	out, err := a.dynamoClient.ListTables(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoDescribeTable(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.dynamoClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(argStr(args, "table_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoGetItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	keyStr := argStr(args, "key")
	var key map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(keyStr, &key); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(argStr(args, "table_name")),
		Key:       key,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoPutItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	itemStr := argStr(args, "item")
	var item map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(itemStr, &item); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(argStr(args, "table_name")),
		Item:      item,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoQuery(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(argStr(args, "table_name")),
		KeyConditionExpression: aws.String(argStr(args, "key_condition_expression")),
	}

	if v := argStr(args, "expression_attribute_values"); v != "" {
		var vals map[string]dynamotypes.AttributeValue
		if err := unmarshalDynamoJSON(v, &vals); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeValues = vals
	}
	if v := argStr(args, "expression_attribute_names"); v != "" {
		var names map[string]string
		if err := json.Unmarshal([]byte(v), &names); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeNames = names
	}
	if v := argStr(args, "index_name"); v != "" {
		input.IndexName = aws.String(v)
	}
	if v := argInt32(args, "limit"); v > 0 {
		input.Limit = aws.Int32(v)
	}
	if argStr(args, "scan_index_forward") == "false" {
		input.ScanIndexForward = aws.Bool(false)
	}

	out, err := a.dynamoClient.Query(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoScan(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(argStr(args, "table_name")),
	}
	if v := argStr(args, "filter_expression"); v != "" {
		input.FilterExpression = aws.String(v)
	}
	if v := argStr(args, "expression_attribute_values"); v != "" {
		var vals map[string]dynamotypes.AttributeValue
		if err := unmarshalDynamoJSON(v, &vals); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeValues = vals
	}
	if v := argStr(args, "expression_attribute_names"); v != "" {
		var names map[string]string
		if err := json.Unmarshal([]byte(v), &names); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeNames = names
	}
	if v := argInt32(args, "limit"); v > 0 {
		input.Limit = aws.Int32(v)
	}
	out, err := a.dynamoClient.Scan(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func dynamoDeleteItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	keyStr := argStr(args, "key")
	var key map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(keyStr, &key); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(argStr(args, "table_name")),
		Key:       key,
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func unmarshalDynamoJSON(s string, out *map[string]dynamotypes.AttributeValue) error {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return err
	}
	result := make(map[string]dynamotypes.AttributeValue, len(raw))
	for k, typeVal := range raw {
		for typeName, val := range typeVal {
			switch typeName {
			case "S":
				var sv string
				if err := json.Unmarshal(val, &sv); err != nil {
					return err
				}
				result[k] = &dynamotypes.AttributeValueMemberS{Value: sv}
			case "N":
				var nv string
				if err := json.Unmarshal(val, &nv); err != nil {
					return err
				}
				result[k] = &dynamotypes.AttributeValueMemberN{Value: nv}
			case "BOOL":
				var bv bool
				if err := json.Unmarshal(val, &bv); err != nil {
					return err
				}
				result[k] = &dynamotypes.AttributeValueMemberBOOL{Value: bv}
			case "NULL":
				result[k] = &dynamotypes.AttributeValueMemberNULL{Value: true}
			case "B":
				var b []byte
				if err := json.Unmarshal(val, &b); err != nil {
					return err
				}
				result[k] = &dynamotypes.AttributeValueMemberB{Value: b}
			default:
				var sv string
				if err := json.Unmarshal(val, &sv); err != nil {
					return err
				}
				result[k] = &dynamotypes.AttributeValueMemberS{Value: sv}
			}
		}
	}
	*out = result
	return nil
}
