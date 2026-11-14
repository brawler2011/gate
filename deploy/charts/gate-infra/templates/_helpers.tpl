{{/* Pod security shared by every pod; appArmorProfile is per environment. */}}
{{- define "gate-infra.podSecurity" -}}
{{- with .Values.appArmorProfile -}}
appArmorProfile: {{- toYaml . | nindent 2 }}
{{ end -}}
seccompProfile: {type: RuntimeDefault}
{{- end -}}
