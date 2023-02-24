import boto3

from simple_lock import acquire_lock, get_lock

dynamodb = boto3.resource('dynamodb')


lock = get_lock(dynamodb, "resource")
print(f"lock: {lock}")
lock_acquired = acquire_lock(dynamodb, "resource", 10, "tx-1")
print(f"lock_acquired: {lock_acquired}")


# Act
lock_acquired = acquire_lock(dynamodb, "resource", 5, "tx-1")
print(f"lock_acquired: {lock_acquired}")


