{{/*
Expand the name of the chart.
*/}}
{{- define "dn-essence.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "dn-essence.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "dn-essence.labels" -}}
helm.sh/chart: {{ include "dn-essence.name" . }}-{{ .Chart.Version }}
{{ include "dn-essence.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "dn-essence.selectorLabels" -}}
app.kubernetes.io/name: {{ include "dn-essence.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
