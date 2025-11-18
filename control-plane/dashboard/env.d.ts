declare namespace NodeJS {
  interface ProcessEnv {
    GOOGLE_CLIENT_ID?: string;
    GOOGLE_CLIENT_SECRET?: string;
    EMAIL_SERVER?: string;
    EMAIL_FROM?: string;
    ADMIN_API_TOKEN?: string;
    CROSSLOGIC_ADMIN_TOKEN?: string;
    CROSSLOGIC_API_BASE_URL?: string;
    CROSSLOGIC_DASHBOARD_TENANT_ID?: string;
    NEXT_PUBLIC_CONTROL_PLANE_URL?: string;
    NEXT_PUBLIC_ADMIN_TOKEN?: string;
  }
}

