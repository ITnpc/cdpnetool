import { TFunction } from 'i18next'

interface ApiResponse {
  success: boolean
  code?: string
  message?: string
}

/**
 * 根据错误码获取国际化错误消息
 */
export function getErrorMessage(error: ApiResponse | string, t: TFunction): string {
  // 如果是字符串，直接返回
  if (typeof error === 'string') {
    return error
  }

  // 如果成功，不处理
  if (error.success) {
    return ''
  }

  // 如果有错误码，使用国际化
  if (error.code && t(`errors.${error.code}`, { defaultValue: '' })) {
    return t(`errors.${error.code}`)
  }

  // 如果有 message，使用 message
  if (error.message) {
    return error.message
  }

  // 默认返回未知错误
  return t('errors.UNKNOWN_ERROR')
}
