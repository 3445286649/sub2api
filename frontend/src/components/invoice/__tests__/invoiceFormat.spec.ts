import { describe, expect, it } from 'vitest'
import { emptyInvoiceTitleForm, formatCny, maskMiddle, orderNumberOf, parseOrderNumbers, refundInvoiceNotice, sumAmounts, validateInvoiceTitle } from '../invoiceFormat'

describe('invoiceFormat', () => {
  it('sums amounts in cents so totals have no float drift', () => {
    expect(sumAmounts([0.1, 0.2])).toBe(0.3)
    expect(sumAmounts([19.99, 0.01, 100])).toBe(120)
    expect(sumAmounts([])).toBe(0)
  })

  it('formats CNY and treats non-finite values as zero', () => {
    expect(formatCny(1234.5)).toBe('¥1,234.50')
    expect(formatCny(undefined)).toBe('¥0.00')
    expect(formatCny(Number.NaN)).toBe('¥0.00')
  })

  it('masks the middle of long values and leaves short ones alone', () => {
    expect(maskMiddle('330106199001011234')).toBe('3301********1234')
    expect(maskMiddle('12345678')).toBe('12345678')
    expect(maskMiddle(null)).toBe('')
  })

  it('prefers the order number shown in order lists and falls back to the recharge code', () => {
    expect(orderNumberOf({ out_trade_no: 'sub2_a', recharge_code: 'PAY-1-1' })).toBe('sub2_a')
    expect(orderNumberOf({ out_trade_no: '', recharge_code: 'PAY-1-1' })).toBe('PAY-1-1')
  })

  it('parses pasted order numbers across separators and removes duplicates', () => {
    expect(parseOrderNumbers('sub2_a')).toEqual(['sub2_a'])
    expect(parseOrderNumbers('  sub2_a\r\nsub2_b\tsub2_c,sub2_d，sub2_e;sub2_f；sub2_g、sub2_h sub2_a\n')).toEqual([
      'sub2_a', 'sub2_b', 'sub2_c', 'sub2_d', 'sub2_e', 'sub2_f', 'sub2_g', 'sub2_h',
    ])
    expect(parseOrderNumbers(' \n\t ,， ')).toEqual([])
  })

  describe('refundInvoiceNotice', () => {
    it('stays silent for orders that are not invoiced', () => {
      expect(refundInvoiceNotice({ invoiced: false, excluded: false })).toBeNull()
    })

    it('tells the admin to reject a request that is still in progress', () => {
      for (const status of ['pending', 'processing'] as const) {
        expect(refundInvoiceNotice({ invoiced: true, excluded: false, request_id: 9, request_status: status }))
          .toEqual({ key: 'invoice.refundNotice.inProgress', params: { id: 9 } })
      }
    })

    it('names the invoice when it was already issued', () => {
      expect(refundInvoiceNotice({ invoiced: true, excluded: false, request_id: 9, request_status: 'completed', invoice_number: '26332' }))
        .toEqual({ key: 'invoice.refundNotice.issued', params: { id: 9, number: '26332' } })
    })

    it('covers orders the admin marked as invoiced outside the system', () => {
      expect(refundInvoiceNotice({ invoiced: true, excluded: true })).toEqual({ key: 'invoice.refundNotice.excluded', params: {} })
    })

    it('prefers the request details when an excluded order also has an open request', () => {
      expect(refundInvoiceNotice({ invoiced: true, excluded: true, request_id: 9, request_status: 'pending' })?.key).toBe('invoice.refundNotice.inProgress')
    })
  })

  describe('validateInvoiceTitle', () => {
    it('requires enterprise name, tax number and receiver email', () => {
      const form = emptyInvoiceTitleForm('')
      expect(validateInvoiceTitle(form)).toEqual({
        enterprise_name: 'enterpriseNameRequired',
        enterprise_tax_number: 'taxNumberRequired',
        receiver_email: 'emailRequired',
      })
    })

    it('accepts 15/18/20 char tax numbers case-insensitively and rejects other lengths', () => {
      const form = { ...emptyInvoiceTitleForm('a@b.cn'), enterprise_name: '某某公司' }
      for (const tax of ['91330106ma2kxxxx7q', '123456789012345', '12345678901234567890']) {
        expect(validateInvoiceTitle({ ...form, enterprise_tax_number: tax })).toEqual({})
      }
      expect(validateInvoiceTitle({ ...form, enterprise_tax_number: '9133010' }).enterprise_tax_number).toBe('taxNumberInvalid')
    })

    it('checks the optional bank account only when filled, ignoring spaces', () => {
      const form = { ...emptyInvoiceTitleForm('a@b.cn'), enterprise_name: 'X', enterprise_tax_number: '91330106MA2KXXXX7Q' }
      expect(validateInvoiceTitle({ ...form, enterprise_bank_account: '6222 0212 3456 7890' })).toEqual({})
      expect(validateInvoiceTitle({ ...form, enterprise_bank_account: '62-22' }).enterprise_bank_account).toBe('bankAccountInvalid')
    })

    it('ignores enterprise fields for individual titles and validates the optional id number', () => {
      const form = { ...emptyInvoiceTitleForm('a@b.cn'), invoice_type: 'individual' as const }
      expect(validateInvoiceTitle(form)).toEqual({ individual_name: 'individualNameRequired' })
      const named = { ...form, individual_name: '张三' }
      expect(validateInvoiceTitle(named)).toEqual({})
      expect(validateInvoiceTitle({ ...named, individual_id_number: '33010619900101123x' })).toEqual({})
      expect(validateInvoiceTitle({ ...named, individual_id_number: '3301061990' }).individual_id_number).toBe('idNumberInvalid')
    })

    it('requires a category only when the caller says categories are configured', () => {
      const form = { ...emptyInvoiceTitleForm('a@b.cn'), invoice_type: 'individual' as const, individual_name: '张三' }
      expect(validateInvoiceTitle(form)).toEqual({})
      expect(validateInvoiceTitle(form, true)).toEqual({ category_id: 'categoryRequired' })
      expect(validateInvoiceTitle({ ...form, category_id: 3 }, true)).toEqual({})
    })

    it('starts without a category selected', () => {
      expect(emptyInvoiceTitleForm('').category_id).toBeNull()
    })

    it('rejects malformed receiver emails', () => {
      const form = { ...emptyInvoiceTitleForm('not-an-email'), invoice_type: 'individual' as const, individual_name: '张三' }
      expect(validateInvoiceTitle(form).receiver_email).toBe('emailInvalid')
    })
  })
})
