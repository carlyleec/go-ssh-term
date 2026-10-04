export const ENDPOINTS = {
  currentUser: '/api/auth/me',
  registerBegin: '/api/auth/register/begin',
  registerFinish: '/api/auth/register/finish',
  loginBegin: '/api/auth/login/begin',
  loginFinish: '/api/auth/login/finish',
  logout: '/api/auth/logout',
  keys: '/api/keys',
  connections: '/api/connections',
} as const
