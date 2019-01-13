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
  args = "build -t chim ."
}

action "tag" {
  uses = "actions/docker/tag@master"
  needs = ["build"]
  args = "-l -s chim keyglitch/chim"
}

action "push" {
  uses = "actions/docker/cli@c08a5fc9e0286844156fefff2c141072048141f6"
  needs = ["tag"]
  args = "push keyglitch/chim"
}
