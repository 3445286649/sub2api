import { describe, expect, it } from 'vitest'
import { selectLegalDocuments } from '@/utils/legalDocuments'
import type { LoginAgreementDocument } from '@/types'

function doc(over: Partial<LoginAgreementDocument> = {}): LoginAgreementDocument {
  return { id: 'terms', title: '服务条款', content_md: '正文', ...over }
}

describe('selectLegalDocuments', () => {
  it('keeps documents that have an id, title and body', () => {
    expect(selectLegalDocuments([doc()])).toHaveLength(1)
  })

  // 后台未配置时后端会返回正文为空的默认协议，展示出来会让用户点进空白页
  it('drops documents without a body', () => {
    expect(selectLegalDocuments([doc({ content_md: '' }), doc({ content_md: '   ' })])).toEqual([])
  })

  it('drops documents without a usable id or title', () => {
    expect(selectLegalDocuments([doc({ id: '' }), doc({ title: '  ' })])).toEqual([])
  })

  it('keeps the original order and ignores invalid input', () => {
    const docs = [doc({ id: 'a', title: 'A' }), doc({ id: 'b', title: 'B' })]

    expect(selectLegalDocuments(docs).map((d) => d.id)).toEqual(['a', 'b'])
    expect(selectLegalDocuments(undefined)).toEqual([])
    expect(selectLegalDocuments(null)).toEqual([])
  })
})
