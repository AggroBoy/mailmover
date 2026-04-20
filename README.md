# MailMover

`mailmover` is a small Go service that watches an IMAP folder and moves messages to another folder.

At startup it loads IMAP credentials and folder names from environment variables or from files under `/etc/mailmover/{config,secrets}`. It processes the source folder immediately, then keeps an IMAP IDLE connection open so new mail can be detected and moved after a short debounce period.

The intent is to watch the 'bin' directory, where various Apple tools unilaterally put messages (including mail, if you use the standard keybindings), and move anything that shows up there into 'Archive'.

As of early 2026, this tool is running 24/7 in my k3s cluster and an important part of my mail handling workflow. It's functional, updated and maintained.

## Configuration

Required settings:

- `IMAP_USERNAME`
- `IMAP_PASSWORD`
- `IMAP_SERVER`
- `FROM_FOLDER`
- `TO_FOLDER`

## Run

```sh
go run .
```

The repo also includes a `Dockerfile`, `Makefile`, and a Kubernetes `manifest.yaml` for deployment.
