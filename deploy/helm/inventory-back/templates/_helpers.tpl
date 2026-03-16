{{- define "inventory-back.name" -}}
inventory-back
{{- end -}}

{{- define "inventory-back.fullname" -}}
{{ include "inventory-back.name" . }}
{{- end -}}
