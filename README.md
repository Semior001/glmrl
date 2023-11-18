# glmrl
tui for listing gitlab merge requests with advanced filters

## install
`go install github.com/Semior001/glmrl/glmrl@latest`

## usage
```
Usage:
  glmrl [OPTIONS] list [list-OPTIONS]

Application Options:
  -c, --config=                             path to config file (default: ~/.glmrl/config.yaml)
      --dbg                                 turn on debug mode [$DEBUG]

gitlab:
      --gitlab.base-url=                    gitlab host [$GITLAB_BASE_URL]
      --gitlab.token=                       gitlab token [$GITLAB_TOKEN]

trace:
      --trace.enabled                       enable tracing [$TRACE_ENABLED]
      --trace.host=                         jaeger agent host [$TRACE_HOST]
      --trace.port=                         jaeger agent port [$TRACE_PORT]

Help Options:
  -h, --help                                Show this help message

[list command options]
          --state=                          list only merge requests with the given state
          --approved-by-me=[true|false|]    list only merge requests approved by me
          --without-my-unresolved-threads   list only merge requests without MY unresolved threads, but lists threads where my action is required
          --not-enough-approvals=           list only merge requests with not enough approvals, but show the ones where I've been requested as a reviewer and didn't approve it
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

If pagination is not specified, it will show all pull requests that match the filters.

## example
```
I can review only the MRs that:
- are open,
- have a label "to-review",
- are not approved by me,
- doesn't have enough approvals,
  - but I still want to see MRs with not enough approvals where I've been requested as a reviewer and didn't approve it,
- doesn't have my unresolved threads
  - but I still want to see MRs with my unresolved threads where my action is not the last (i.e. somebody replied in my thread and didn't resolve it).

When I press enter, I want to open the MR in my default browser.
```

command example:
```bash
glmrl list --action=open --state=open --labels.include='to-review' --approved-by-me=false --not-enough-approvals=true --without-my-unresolved-threads
```
