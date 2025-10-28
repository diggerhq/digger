{{/*
Expand the name of the chart.
*/}}
{{- define "opentaco.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "opentaco.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "opentaco.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "opentaco.labels" -}}
helm.sh/chart: {{ include "opentaco.chart" . }}
{{ include "opentaco.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "opentaco.selectorLabels" -}}
app.kubernetes.io/name: {{ include "opentaco.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Database host - returns the appropriate database host based on configuration
*/}}
{{- define "opentaco.database.host" -}}
{{- if .Values.cloudSql.enabled -}}
127.0.0.1
{{- else if .Values.postgresql.enabled -}}
{{ .Release.Name }}-postgresql
{{- else -}}
{{- required "Either postgresql.enabled or cloudSql.enabled must be true, or provide external database host" .Values.externalDatabase.host -}}
{{- end -}}
{{- end }}

{{/*
Database port
*/}}
{{- define "opentaco.database.port" -}}
{{- if .Values.cloudSql.enabled -}}
5432
{{- else if .Values.postgresql.enabled -}}
5432
{{- else -}}
{{ .Values.externalDatabase.port | default 5432 }}
{{- end -}}
{{- end }}

