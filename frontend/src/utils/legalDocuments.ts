import type { LoginAgreementDocument } from '@/types'

/**
 * 挑选可在页脚 / 登录页展示的协议文档。
 *
 * 后端在后台未配置协议时会返回一组内置文档，但它们的 content_md 为空，
 * 前台直接展示会让用户点进空白页，所以这里只保留有正文的条目。
 */
export function selectLegalDocuments(
  documents: LoginAgreementDocument[] | undefined | null
): LoginAgreementDocument[] {
  if (!Array.isArray(documents)) {
    return []
  }
  return documents.filter((doc) => Boolean(doc?.id && doc.title?.trim() && doc.content_md?.trim()))
}
