#!/bin/bash

# Enable services
gcloud services enable compute.googleapis.com --project=proxy-v2-test
gcloud services enable servicenetworking.googleapis.com --project=proxy-v2-test
gcloud services enable sqladmin.googleapis.com --project=proxy-v2-test

# Create a vnet
gcloud compute networks create proxy-v2-testnet-100 \
  --project=proxy-v2-test \
  --description=Test\ network\ 100 \
  --subnet-mode=auto --mtu=1460 \
  --bgp-routing-mode=regional

# Allow the internet to port 22
gcloud compute --project=proxy-v2-test firewall-rules create allow-ssh \
  --direction=INGRESS \
  --priority=1000 \
  --network=proxy-v2-testnet-100 \
  --action=ALLOW --rules=tcp:22 --source-ranges=104.132.147.66/32

# Enable service connections for google services
#    part 1: allocate an ip range
gcloud compute addresses create google-managed-services-my-network \
    --global \
    --purpose=VPC_PEERING \
    --prefix-length=24 \
    --description="peering range for Google" \
    --network=proxy-v2-testnet-100 \
    --project=proxy-v2-test
#   part 2: enable peering
gcloud services vpc-peerings update \
    --service=servicenetworking.googleapis.com \
    --ranges=google-managed-services-my-network \
    --network=proxy-v2-testnet-100 \
    --project=proxy-v2-test


# Create VM
gcloud compute instances create instance-3 \
  --project=proxy-v2-test \
  --zone=us-central1-a \
  --machine-type=e2-micro \
  --network-interface=network-tier=PREMIUM,subnet=proxy-v2-testnet-100 \
  --maintenance-policy=MIGRATE --provisioning-model=STANDARD \
  --service-account=860730149301-compute@developer.gserviceaccount.com \
  --scopes=https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring.write,https://www.googleapis.com/auth/servicecontrol,https://www.googleapis.com/auth/service.management.readonly,https://www.googleapis.com/auth/trace.append,https://www.googleapis.com/auth/sqlservice.admin --create-disk=auto-delete=yes,boot=yes,device-name=instance-2,image=projects/debian-cloud/global/images/debian-11-bullseye-v20220621,mode=rw,size=10,type=projects/proxy-v2-test/zones/us-central1-a/diskTypes/pd-balanced \
  --no-shielded-secure-boot --shielded-vtpm --shielded-integrity-monitoring --reservation-affinity=any


# Build a linux-amd64 binary
GOOS=linux GOARCH=amd64 go build -o proxy-linux-amd64 main.go

# Copy the proxy to the machine
vm_name=instance-3
gcloud compute scp --zone "us-central1-a"   --project "proxy-v2-test" "./proxy-linux-amd64" "instance-3:proxy"

# SSH to the machine
gcloud compute ssh --zone "us-central1-a" "instance-3"  --project "proxy-v2-test"

# Start the proxy
./proxy --private-ip proxy-v2-test:us-central1:testmysql &

# Install and run mysql client
sudo apt-get update && sudo apt-get install -y mariadb-client
mysql --host 127.0.0.1 --port 3306 --user root --password