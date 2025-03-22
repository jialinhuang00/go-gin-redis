#!/bin/bash

set -e

ECR_REPO_NAME="golang/gin-redis"
AWS_REGION="ap-northeast-1"
AWS_ACCOUNT_ID="263386971159"
ECS_CLUSTER_NAME="gin-redis-cluster"
ECS_SERVICE_NAME="gin-redis-service"

# check or create ecr repo
if ! aws ecr describe-repositories --repository-names $ECR_REPO_NAME --region $AWS_REGION &> /dev/null; then
    echo "Creating ECR repository: $ECR_REPO_NAME"
    aws ecr create-repository --repository-name $ECR_REPO_NAME --region $AWS_REGION
else
    echo "ECR repository $ECR_REPO_NAME already exists"
fi

# login to ECR
echo "Logging in to ECR"
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

# create image
echo "Building Docker image"
docker build --platform linux/amd64 -t $ECR_REPO_NAME .

# tag image
echo "Tagging Docker image"
docker tag $ECR_REPO_NAME:latest $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:latest

# push to ecr
echo "Pushing image to ECR"
docker push $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com/$ECR_REPO_NAME:latest

# check or create ecs cluster
if ! aws ecs describe-clusters --clusters $ECS_CLUSTER_NAME --region $AWS_REGION | grep -q "ACTIVE"; then
    echo "Creating ECS cluster: $ECS_CLUSTER_NAME"
    aws ecs create-cluster --cluster-name $ECS_CLUSTER_NAME --region $AWS_REGION
else
    echo "ECS cluster $ECS_CLUSTER_NAME already exists"
fi

# check if service is already existed.
if aws ecs describe-services --cluster $ECS_CLUSTER_NAME --services $ECS_SERVICE_NAME --region $AWS_REGION | grep -q "ACTIVE"; then
    echo "Updating ECS service: $ECS_SERVICE_NAME"
    aws ecs update-service --cluster $ECS_CLUSTER_NAME --service $ECS_SERVICE_NAME --force-new-deployment --region $AWS_REGION
else
    echo "ECS service $ECS_SERVICE_NAME does not exist. Please create it manually."
fi

echo "Deployment script completed"