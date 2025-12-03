This repository holds code that demonstrates a progression of ever more sophisticated use cases for orchestrating Terraform. We use Temporal with golang for the orchestration.

This was orginally developed for a workshop delivered at PlatformCon 2025 (live in London and virtual) (will add links the the recordings when available).

# Application overview

This code orchestrates the provisioning and management of an AWS EC2 instance and then a Cloudflare DNS record for the newly created server. It does support multiple running environments with a very simple state file storage mechanism - it stores them in subdirectories under `terraform-configs/*-terraform/state`.

Please see the session recordings (coming soon) for more details.

# Running the demos

## prerequisites

- temporal cli installed
- terraform cli installed
- AWS CLI installed and configured with SSO profiles
- You will need a domain to which you can add DNS entries in Cloudflare

### AWS Authentication Setup

This project uses AWS SSO for authentication. Before running the application:

1. **Authenticate with AWS SSO** using one of your configured profiles:
   ```bash
   aws sso login --profile <your AWS profile>
   ```

2. **Set the AWS_PROFILE environment variable** to specify which profile to use:
   ```bash
   export AWS_PROFILE=<your AWS profile>
   ```

   Note: If `AWS_PROFILE` is not set, Terraform will use the default AWS credential chain (which may include your default profile or other configured credentials).

3. **Set Cloudflare environment variables**:
   ```bash
   export CLOUDFLARE_ZONE_ID="..."
   export CLOUDFLARE_API_TOKEN="..."
   ```

**Important**: AWS SSO sessions typically expire after a few hours. If you encounter authentication errors, re-authenticate with `aws sso login --profile <your-profile>`.

You will need three command windows. 

In the first, run the Temporal service locally with `temporal server start-dev`. You will then find the Temporal UI at [http://localhost:8233/](http://localhost:8233/)

You will use the other two to perform the demos

## Execution

### demo 1: Basic orchestration

1. `git checkout demo1`
2. Run the orchstration in the first terminal: `go run ./cmd/worker/main.go`
3. Run the starter in the second terminal: `go run ./cmd/starter/main.go -action=create`
4. To destroy `go run ./cmd/starter/main.go -action=destroy -environment=env-id`
5. Show the workflow in the UI - just the two activity calls.
6. Show the workflow code.

### demo 2: Add Human in the Loop (HITL)

1. `git checkout demo2`
2. Run the orchstration: `go run ./cmd/worker/main.go`
3. Run the starter: `go run ./cmd/starter/main.go -action=create -dns-approved=false`
	1. To approve `go run ./cmd/starter/main.go -action=approve -environment=env-id`
	2. To destroy `go run ./cmd/starter/main.go -action=destroy -environment=env-id`
4. In the UI show the signal coming in.
5. In the code show the selectors that allow for signals to come into the workflow

### demo 2-b: Show durability

1. Kill the orchestration while it's waiting on approval. Let the timer fire. Bring the orchestration back.
2. Use the `shouldfail` flag in the activity. If you do it for DNS you'll have the opportunity to point out that the AWS infra provisioning is not retried.

### demo 3: Add a timeout on the approval (Durable timer)

1. `git checkout demo3`
2. Run the orchstration: `go run ./cmd/worker/main.go`
3. Run the starter: `go run ./cmd/starter/main.go -action=create -dns-approved=false`
	1. To approve `go run ./cmd/starter/main.go -action=approve -environment=env-id`
	2. To destroy `go run ./cmd/starter/main.go -action=destroy -environment=env-id`
4. Show the timer
5. Show the durability of the timer by running again and killing the orchstration right after the timer starts.

### demo 3-b: Talk about deterministic and idempotent

1. Start a creation: `go run ./cmd/starter/main.go -action=create`
2. Kill the orchstration before the AWS provisioning is finished.
3. Restart the orchstration
4. After the start to close timeout, it will retry which doesn't recreate because terraform is usually idempotent.

### demo 4: The pièce de résistance! Digital Twin!!! (Entity workflows)

1. Start a creation: `go run ./cmd/starter/main.go -action=create` 
    1. Note that after the steps finish the workfow is still running
2. Update: `go run ./cmd/starter/main.go -action=update -environment=env-id`
    1. if done with no change to the `main.tf` it will be a no op.
    2. if ami is changed in the `main.tf` terraform will cause old instance to shut down and a new one to be created.
3. To approve `go run ./cmd/starter/main.go -action=approve -environment=env-id`
4. To destroy `go run ./cmd/starter/main.go -action=destroy -environment=env-id`
    1. this will end the workflow (after deprovisioning)


## Simulating a network outage

The implementation of the `get_forecast` tool includes a 10 second sleep between the two HTTP requests. Experiment with the following:
- Run it with no firewall rules
- Add the firewall rules and enable the firewall
- Disable the firewall, accept the MCP tool execution and then enable the firewall within 10 seconds. Disable the firewall on the 11th second and see what happens.


### Using `pfctl` on a Mac

We will simulate a network outage by adding firewall rules using `pfctl`. This repository includes a `pf.rules` file that will allow you to block API access to Cloudflare. The Cloudflare DNS resolution is fairly stable (some ther APIs like the National Weather Service are not) so the pf.rules file currently just has the domain name. You can check what these are right now with the following command:
```
dig +short api.cloudflare.com
```

The following commands are used to set and delete the rules, and enable and disable the firewall.

To set rules
```
sudo pfctl -f pf.rules
```

To remove the rules. WARNING: this will delete all rules - if you are using pfctl for real, use with caution.
```
sudo pfctl -F all
```

To see the current list of rules:
```
sudo pfctl -s rules
```

To enable the firewall
```
sudo pfctl -e
```

To disable the firewall
```
sudo pfctl -d
```
