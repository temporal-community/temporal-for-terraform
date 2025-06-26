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
}

resource "aws_instance" "app_server" {
  # Using a different Ubuntu 22.04 LTS AMI to force instance replacement on update
  # Previous AMI:   ami-0735c191cf914754d
  # Previous AMI:   ami-08d70e59c07c61a3a
  ami           = "ami-08d70e59c07c61a3a"
  instance_type = "t2.micro"

  tags = {
    Name = "ExampleAppServerInstance"
  }
}

output "web_server_public_ip" {
  value = aws_instance.app_server.public_ip
}