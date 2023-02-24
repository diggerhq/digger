import boto3

from simple_lock import get_lock

# get a reference to the DynamoDB resource
dynamodb = boto3.resource('dynamodb')



lock = get_lock(dynamodb, "resource", "tx-1")
print(f"lock: {lock}")



