{{/* Pod security shared by every pod; appArmorProfile is per environment. */}}
{{- define "gate-infra.podSecurity" -}}
appArmorProfile: {{- toYaml .Values.appArmorProfile | nindent 2 }}
seccompProfile: {type: RuntimeDefault}
{{- end -}}
