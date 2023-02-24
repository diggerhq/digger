import boto3

from simple_lock import release_lock, get_lock

dynamodb = boto3.resource('dynamodb')

lock = get_lock(dynamodb, "test_github_actions", "tx-1")
print(f'lock: {lock}')
lock_released = release_lock(dynamodb, "test_github_actions", "tx-1")

print(f"lock_released: {lock_released}")