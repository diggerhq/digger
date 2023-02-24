import boto3

from simple_lock import acquire_lock, get_lock

dynamodb = boto3.resource('dynamodb')


lock = get_lock(dynamodb, "resource", "tx-1")
print(f"lock: {lock}")

lock_acquired = acquire_lock(dynamodb, "resource", 10, "tx-1")
print(f"lock_acquired: {lock_acquired}")

lock = get_lock(dynamodb, "resource", "tx-1")
print(f"lock: {lock}")
print(f"lock id: {lock['transaction_id']}")