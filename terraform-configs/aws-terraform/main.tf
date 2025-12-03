terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.16"
    }
  }

  required_version = ">= 1.2.0"

  backend "local" {}
}

provider "aws" {
  region = "us-west-2"
  # For AWS SSO, authenticate first with: aws sso login --profile <profile-name>
}

resource "aws_instance" "app_server" {
  ami           = "ami-0ffde298a37fd43b7"
  instance_type = "t2.micro"

  metadata_options {
    http_tokens   = "required"  # Required by SCP to use IMDSv2
    http_endpoint = "enabled"
  }

  tags = {
    Name = "ExampleAppServerInstance"
  }
}

output "web_server_public_ip" {
  value = aws_instance.app_server.public_ip
}