workflow "Image Build" {
  on = "push"
  resolves = ["push"]
}

action "login" {
  uses = "actions/docker/login@c08a5fc9e0286844156fefff2c141072048141f6"
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "build" {
  uses = "actions/docker/cli@c08a5fc9e0286844156fefff2c141072048141f6"
  needs = ["login"]
  args = "build -t keyglitch/chim ."
}

action "tag" {
  uses = "actions/docker/tag@master"
  needs = ["build"]
  args = "keyglitch/chim keyglitch/chim:$GITHUB_SHA"
}

action "push" {
  uses = "actions/docker/cli@c08a5fc9e0286844156fefff2c141072048141f6"
  needs = ["tag"]
  args = "push keyglitch/chim"
}
