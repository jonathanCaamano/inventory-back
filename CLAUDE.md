# CLAUDE.md — Instructions for Claude Code

## CI local

Ejecutar **antes de cada commit** para evitar que lleguen errores a GitHub Actions:

```bash
gofmt -l .                      # no debe mostrar ficheros
go vet ./...
golangci-lint run --timeout=5m
go test -race ./...
go build -o /dev/null ./cmd/server
```

## Git

- Ramas: `feature/`, `bugfix/`, `hotfix/`, `release/` — sin prefijos adicionales
- Commits: convencional (`feat:`, `fix:`, `chore:`, etc.) — sin mencionar herramientas externas ni agentes en el mensaje
- PRs: título y descripción propios del cambio — sin mencionar herramientas externas ni agentes
- Comentarios y documentación: redactar en primera persona del equipo — sin atribuir autoría a herramientas

## Git commits
- Never include session URLs (e.g. `https://claude.ai/code/session_...`) in commit messages.
