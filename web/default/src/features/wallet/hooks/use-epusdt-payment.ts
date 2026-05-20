/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState, useCallback } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { requestEpusdtPayment, isApiSuccess } from '../api'

function getPaymentUrl(data: unknown): string | null {
  if (!data || typeof data !== 'object') return null
  if ('payment_url' in data && typeof data.payment_url === 'string') {
    return data.payment_url
  }
  return null
}

function isSafeHttpUrl(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return false
  try {
    const u = new URL(trimmed)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

function getErrorMessage(message: string | undefined, data: unknown): string {
  if (typeof data === 'string' && data.trim()) return data
  return message || i18next.t('Payment request failed')
}

/**
 * Hook for handling Epusdt payment processing.
 * The backend returns a `payment_url` (Epusdt checkout page) which we open in a new tab.
 */
export function useEpusdtPayment() {
  const [processing, setProcessing] = useState(false)

  const processEpusdtPayment = useCallback(
    async (topupAmount: number, paymentMethod: string) => {
      setProcessing(true)
      try {
        const response = await requestEpusdtPayment({
          amount: Math.floor(topupAmount),
          payment_method: paymentMethod,
        })

        if (isApiSuccess(response)) {
          const paymentUrl = getPaymentUrl(response.data)
          if (paymentUrl) {
            if (!isSafeHttpUrl(paymentUrl)) {
              toast.error(i18next.t('Invalid payment redirect URL'))
              return false
            }
            window.open(paymentUrl, '_blank', 'noopener,noreferrer')
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        toast.error(getErrorMessage(response.message, response.data))
        return false
      } catch (_error) {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return { processing, processEpusdtPayment }
}
