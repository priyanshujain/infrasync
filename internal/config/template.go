package config

const Template = `
name: {{ project_name }}
path: {{ project_path }}

providers:
  google:
    credentials: {{ gcp_credentials_path }}
    projects:
      - id: {{ gcp_project_id }}
        region: {{ gcp_region }}
        services:
          {{- range gcp_services }}
          - {{ . }}
          {{- end }}

backend:
  type: {{ backend_type }}
  bucket: {{ backend_bucket }}
`
