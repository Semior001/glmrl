# glmrl
tui for listing gitlab merge requests with advanced filters

## install
`go install github.com/Semior001/glmrl/glmrl@latest`

## usage
```
Usage:
  glmrl [OPTIONS] list [list-OPTIONS]

Application Options:
  -c, --config=                             path to config file (default: ~/.cloudcli/config.yaml)
      --dbg                                 turn on debug mode [$DEBUG]

gitlab:
      --gitlab.base-url=                    gitlab host [$GITLAB_BASE_URL]
      --gitlab.token=                       gitlab token [$GITLAB_TOKEN]

Help Options:
  -h, --help                                Show this help message

[list command options]
          --state=                          list only merge requests with the given state
          --approved-by-me=[true|false|]    list only merge requests approved by me
          --without-my-unresolved-threads   list only merge requests without MY unresolved threads
          --not-enough-approvals=           list only merge requests with not enough approvals
          --action=[open|copy]              action to perform on pressing enter (default: copy)
          --poll-interval=                  interval to poll for new merge requests (default: 5m)

    labels:
          --labels.include=                 list only entries that include the given value
          --labels.exclude=                 list only entries that exclude the given value

    authors:
          --authors.include=                list only entries that include the given value
          --authors.exclude=                list only entries that exclude the given value

    project-paths:
          --project-paths.include=          list only entries that include the given value
          --project-paths.exclude=          list only entries that exclude the given value

    sort:
          --sort.by=[created|updated|title] sort by the given field (default: created)
          --sort.order=[asc|desc]           sort in the given order (default: desc)

    pagination:
          --pagination.page=                page number
          --pagination.per-page=            number of items per page
```

## example
"I can review only the MRs that":
- are open,
- have a label "to-review",
- are not approved by me,
- doesn't have enough approvals,
- doesn't have my unresolved threads.

command example:
```bash
glmrl --state=open --labels.include='to-review' --approved-by-me=false --not-enough-approvals=true --without-my-unresolved-threads
```
