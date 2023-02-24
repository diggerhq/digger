import typing

from datetime import datetime, timedelta
import boto3
from boto3.dynamodb import conditions

TABLE_NAME = "DiggerDynamoDBLockTable"


def create_table_if_not_exists(dynamodb_client, table_name):
    try:
        response = dynamodb_client.create_table(
            AttributeDefinitions=[
                {
                    'AttributeName': 'timeout',
                    'AttributeType': 'S',
                },
                {
                    'AttributeName': 'transaction_id',
                    'AttributeType': 'N',
                },
            ],
            KeySchema=[],
            ProvisionedThroughput={
                'ReadCapacityUnits': 5,
                'WriteCapacityUnits': 5,
            },
            TableName='test',
        )
    except dynamodb_client.exceptions.ResourceInUseException:
        # do something here as you require
        pass    

def acquire_lock(
    dynamodb_client, resource_name: str, timeout_in_seconds: int, transaction_id: str
) -> bool:

    ex = dynamodb_client.meta.client.exceptions
    table = dynamodb_client.Table(TABLE_NAME)

    now = datetime.now().isoformat(timespec="seconds")
    new_timeout = (datetime.now() + timedelta(seconds=timeout_in_seconds)).isoformat(
        timespec="seconds"
    )

    try:
        result = table.update_item(
            Key={"PK": "LOCK", "SK": f"RES#{resource_name}"},
            UpdateExpression="SET #tx_id = :tx_id, #timeout = :timeout",
            ExpressionAttributeNames={
                "#tx_id": "transaction_id",
                "#timeout": "timeout",
            },
            ExpressionAttributeValues={
                ":tx_id": transaction_id,
                ":timeout": new_timeout,
            },
            ConditionExpression=conditions.Or(
                conditions.Attr("SK").not_exists(),  # New Item, i.e. no lock
                conditions.Attr("timeout").lt(now),  # Old lock is timed out
            ),
        )
        # print(f"update_item: {result}")
        return True

    except ex.ConditionalCheckFailedException as e:
        # print(e)
        # It's already locked
        return False


def release_lock(dynamodb_client, resource_name: str, transaction_id: str) -> bool:
    table = dynamodb_client.Table(TABLE_NAME)
    ex = dynamodb_client.meta.client.exceptions

    try:
        table.delete_item(
            Key={"PK": "LOCK", "SK": f"RES#{resource_name}"},
            #            ConditionExpression=conditions.Attr("transaction_id").eq(transaction_id),
        )
        return True

    except (ex.ConditionalCheckFailedException, ex.ResourceNotFoundException) as e:
        print(e)
        return False


def get_lock(dynamodb_client, resource_name: str):
    table = dynamodb_client.Table(TABLE_NAME)

    item = table.get_item(
        Key={"PK": "LOCK", "SK": f"RES#{resource_name}"},
    )
    if "Item" in item:
        return item["Item"]
    else:
        return None


def create_table_if_not_exists():

    client = boto3.client("dynamodb")

    try:
        client.create_table(
            AttributeDefinitions=[
                {"AttributeName": "PK", "AttributeType": "S"},
                {"AttributeName": "SK", "AttributeType": "S"},
            ],
            TableName=TABLE_NAME,
            KeySchema=[
                {"AttributeName": "PK", "KeyType": "HASH"},
                {"AttributeName": "SK", "KeyType": "RANGE"},
            ],
            BillingMode="PAY_PER_REQUEST",
        )

        boto3.resource("dynamodb").Table(TABLE_NAME).wait_until_exists()
    except client.exceptions.ResourceInUseException as e:
        print(e)
