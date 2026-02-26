package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	mcp "github.com/daltoniam/switchboard"
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
			av, err := unmarshalAttributeValue(typeName, val)
			if err != nil {
				return fmt.Errorf("attribute %q: %w", k, err)
			}
			result[k] = av
		}
	}
	*out = result
	return nil
}

func unmarshalAttributeValue(typeName string, val json.RawMessage) (dynamotypes.AttributeValue, error) {
	switch typeName {
	case "S":
		var sv string
		if err := json.Unmarshal(val, &sv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberS{Value: sv}, nil
	case "N":
		var nv string
		if err := json.Unmarshal(val, &nv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberN{Value: nv}, nil
	case "BOOL":
		var bv bool
		if err := json.Unmarshal(val, &bv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberBOOL{Value: bv}, nil
	case "NULL":
		return &dynamotypes.AttributeValueMemberNULL{Value: true}, nil
	case "B":
		var b []byte
		if err := json.Unmarshal(val, &b); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberB{Value: b}, nil
	case "SS":
		var ss []string
		if err := json.Unmarshal(val, &ss); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberSS{Value: ss}, nil
	case "NS":
		var ns []string
		if err := json.Unmarshal(val, &ns); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberNS{Value: ns}, nil
	case "BS":
		var bs [][]byte
		if err := json.Unmarshal(val, &bs); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberBS{Value: bs}, nil
	case "L":
		var rawList []map[string]json.RawMessage
		if err := json.Unmarshal(val, &rawList); err != nil {
			return nil, err
		}
		list := make([]dynamotypes.AttributeValue, 0, len(rawList))
		for _, item := range rawList {
			for tn, tv := range item {
				av, err := unmarshalAttributeValue(tn, tv)
				if err != nil {
					return nil, err
				}
				list = append(list, av)
			}
		}
		return &dynamotypes.AttributeValueMemberL{Value: list}, nil
	case "M":
		var rawMap map[string]map[string]json.RawMessage
		if err := json.Unmarshal(val, &rawMap); err != nil {
			return nil, err
		}
		m := make(map[string]dynamotypes.AttributeValue, len(rawMap))
		for mk, mv := range rawMap {
			for tn, tv := range mv {
				av, err := unmarshalAttributeValue(tn, tv)
				if err != nil {
					return nil, err
				}
				m[mk] = av
			}
		}
		return &dynamotypes.AttributeValueMemberM{Value: m}, nil
	default:
		return nil, fmt.Errorf("unsupported DynamoDB type: %s", typeName)
	}
}
