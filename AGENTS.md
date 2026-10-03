# AGENTS

You're working on a software update and release management software for small devices called ondOTA.

## Instructions
- Use Golang for backend service implementation
- follow hexagonal architecture for the backend
- For local development you can use my k3s cluster accessible via kubectl. You can only create/modify/delete kubernetes resource in the namespace `ondota`. DO NOT perform any destructive actions in other namespaces
- This computer doesn't have enough resources to run docker, so you can use the k3s cluster and the `ondota` namespace to deploy a data base or other components that you need
- the k3s cluster is running on the server that has the domain name ownerofglory.com if you need to connect.
- for the backend architecture use of on my projects billpiggy: https://github.com/ownerofglory/billpiggy. You can use it as a reference for the folder, file structure but also DB migrations, Helm chart and Github Actions Pipeline


