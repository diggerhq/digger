# AWS

Complete AWS documentation coming soon



## Setting up locking DynamoDB Access

Digger runs without a backend but uses a DynamoDB table to keep track of all the locks that are necessary for locking PR projects. On the first run in your AWS account digger checks for the presense of `DiggerDynamoDBLockTable` and it requires the following policy for the DynamoDB access:



```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "ListAndDescribe",
            "Effect": "Allow",
            "Action": [
                "dynamodb:List*",
                "dynamodb:DescribeReservedCapacity*",
                "dynamodb:DescribeLimits",
                "dynamodb:DescribeTimeToLive"
            ],
            "Resource": "*"
        },
        {
            "Sid": "SpecificTable",
            "Effect": "Allow",
            "Action": [
                "dynamodb:BatchGet*",
                "dynamodb:DescribeStream",
                "dynamodb:DescribeTable",
                "dynamodb:Get*",
                "dynamodb:Query",
                "dynamodb:Scan",
                "dynamodb:BatchWrite*",
                "dynamodb:CreateTable",
                "dynamodb:Delete*",
                "dynamodb:Update*",
                "dynamodb:PutItem",
                "dynamodb:CreateTable"
            ],
            "Resource": "arn:aws:dynamodb:*:*:table/DiggerDynamoDBLockTable"
        }
    ]
}
```
