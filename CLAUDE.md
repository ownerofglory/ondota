# CLAUDE

You're working on a software update and release management software for small devices called ondOTA.

## Instructions
- Use Golang for backend service implementation
- follow hexagonal architecture for the backend
- For local development you can use my k3s cluster accessible via kubectl. You can only create/modify/delete kubernetes resource in the namespace `ondota`. DO NOT perform any destructive actions in other namespaces
- This computer doesn't have enough resources to run docker, so you can use the k3s cluster and the `ondota` namespace to deploy a data base or other components that you need
- the k3s cluster is running on the server that has the domain name ownerofglory.com if you need to connect.
- for the backend architecture use of on my projects billpiggy: https://github.com/ownerofglory/billpiggy. You can use it as a reference for the folder, file structure but also DB migrations, Helm chart and Github Actions Pipeline
- Prefer event-driven style of architecture but avoid heavy-weight solutions like Apache Kafka.
- For features you implement write unit and integration tests. 
- Backend and eventually client should have a full ci/cd cycle. You can resuse most of the Billpiggy's CI actions with modifications.
- write consicely documented go code with godocs.
- Do not overengineer. if there is a well-suited opensource free soulution for a particular problem you can use it.
- Manager change log with concise entries about implemented features, bug fixes etc. manager parallel log with concise but more detailed info about your work that agents can use to pick up from where one stopped.
- code for both backend and client should be in this repository.

