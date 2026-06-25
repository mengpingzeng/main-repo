/** 是否为开发环境（npm run dev）；生产构建（npm run start）为 false */
export const IS_DEV_RUNTIME = process.env.NODE_ENV === "development"

/** 生产环境内测限制 */
export const PROD_MAX_USERS = 9
export const PROD_MAX_ACCOUNTS_PER_PLATFORM = 1

/** 生产环境是否启用内测人数/绑定限制 */
export function isBetaLimitsEnabled(): boolean {
  return !IS_DEV_RUNTIME
}

export function isUserLimitReached(total: number): boolean {
  return isBetaLimitsEnabled() && total >= PROD_MAX_USERS
}

export function isPlatformAccountLimitReached(accountCount: number): boolean {
  return isBetaLimitsEnabled() && accountCount >= PROD_MAX_ACCOUNTS_PER_PLATFORM
}

export function platformAccountLimitMessage(
  plt: string,
  platformLabels: Record<string, string>,
): string {
  const label = platformLabels[plt] || plt
  return PROD_MAX_ACCOUNTS_PER_PLATFORM === 1
    ? `${label}已绑定账号，每个平台只能绑定 1 个`
    : `${label}最多绑定 ${PROD_MAX_ACCOUNTS_PER_PLATFORM} 个账号`
}
