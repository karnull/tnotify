# Tnotify

Notifications and questions in a tmux popup, for panes nobody is watching



[![Copier](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/copier-org/copier/master/img/badge/badge-grayscale-inverted-border-orange.json)](https://github.com/karshosphere/template-go)

## Setup

### Requirements

* Add system requirements here



### Build

```bash
$ make build
```

### Run

```bash
$ ./build/tnotify
```



## Testing

### Run all tests

```bash
$ make test
```

### Formatting

```bash
$ make format           # format in place
$ make format-dry-run   # list files that need formatting
```




## Documentation

* [Project Structure](docs/project_structure.md): repository layout and module responsibilities




## Release

`make release` builds the project for each release target chosen when it was generated (linux-arm64, linux-x64, macOS-intel, macOS-apple-silicon), writing one binary per target into `dist/`. With no targets, `dist/` is left empty.

The targets are set in the `Makefile`, in `RELEASE_TARGETS`, and built by `scripts/release.sh`.



## Copier Update

The project uses [Copier](https://copier.readthedocs.io/en/stable/) to manage its template. When the template is updated, the project can be easily brought up to date by running the following command from the project root:

```bash
$ copier update --defaults --answers-file .copier-go-answers.yml
```
When the `--defaults` option is used, copier update skips all previously answered questions and prompts only for newly introduced ones.

