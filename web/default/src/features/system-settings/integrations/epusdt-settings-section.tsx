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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useUpdateOption } from '../hooks/use-update-option'

export interface EpusdtSettingsValues {
  EpusdtEnabled: boolean
  EpusdtApiUrl: string
  EpusdtPid: string
  EpusdtSecretKey: string
  EpusdtCurrency: string
  EpusdtTradeTypes: string
}

interface Props {
  defaultValues: EpusdtSettingsValues
}

const CURRENCIES = ['cny', 'usd', 'eur', 'gbp', 'jpy']

function parseNetworkTypes(value: string): string[] {
  return [
    ...new Set(
      value
        .replace(/[;,]/g, '\n')
        .split('\n')
        .map((item) => item.trim())
        .filter(Boolean)
    ),
  ]
}

export function EpusdtSettingsSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [loading, setLoading] = useState(false)
  const form = useForm<EpusdtSettingsValues>({
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [props.defaultValues, form])

  const handleSave = async () => {
    const values = form.getValues()
    const enabled = !!values.EpusdtEnabled

    if (enabled && !values.EpusdtApiUrl.trim()) {
      toast.error(t('Epusdt API URL is required'))
      return
    }
    if (enabled && !/^https?:\/\//.test(values.EpusdtApiUrl.trim())) {
      toast.error(t('Epusdt API URL must start with http:// or https://'))
      return
    }
    if (enabled && !values.EpusdtPid.trim()) {
      toast.error(t('Epusdt PID is required'))
      return
    }
    const currency = CURRENCIES.includes(values.EpusdtCurrency)
      ? values.EpusdtCurrency
      : 'cny'
    const tradeTypes = parseNetworkTypes(values.EpusdtTradeTypes)
    if (enabled && tradeTypes.length === 0) {
      toast.error(t('Required'))
      return
    }

    setLoading(true)
    try {
      const options: { key: string; value: string }[] = [
        { key: 'EpusdtEnabled', value: enabled ? 'true' : 'false' },
        {
          key: 'EpusdtApiUrl',
          value: values.EpusdtApiUrl.trim().replace(/\/$/, ''),
        },
        {
          key: 'EpusdtPid',
          value: values.EpusdtPid.trim(),
        },
        {
          key: 'EpusdtCurrency',
          value: currency,
        },
        {
          key: 'EpusdtTradeTypes',
          value: tradeTypes.join('\n'),
        },
      ]

      // Only send secret if non-empty (avoid clearing existing value with blank)
      if (values.EpusdtSecretKey.trim()) {
        options.push({
          key: 'EpusdtSecretKey',
          value: values.EpusdtSecretKey.trim(),
        })
      }

      for (const option of options) {
        await updateOption.mutateAsync(option)
      }
      toast.success(t('Updated successfully'))
    } catch {
      toast.error(t('Update failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className='space-y-4 pt-4'>
      <div>
        <h3 className='text-lg font-medium'>
          {t('Epusdt Payment Gateway')}
        </h3>
        <p className='text-muted-foreground text-sm'>
          {t('Configure Epusdt crypto payment gateway')}
        </p>
      </div>

      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Deploy Epusdt and fill in the API URL, PID, and secret key below. Callback URL: <ServerAddress>/api/user/epusdt/notify'
          )}
        </AlertDescription>
      </Alert>

      <div className='flex items-center gap-2'>
        <Switch
          checked={form.watch('EpusdtEnabled')}
          onCheckedChange={(v) => form.setValue('EpusdtEnabled', v)}
        />
        <Label>{t('Enable Epusdt')}</Label>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Epusdt Service URL')}</Label>
        <Input
          placeholder='https://epusdt.example.com'
          {...form.register('EpusdtApiUrl')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'The base URL of your Epusdt service (without trailing slash). The order creation endpoint will be appended automatically.'
          )}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Epusdt PID')}</Label>
        <Input placeholder='1000' {...form.register('EpusdtPid')} />
        <p className='text-muted-foreground text-xs'>
          {t('The merchant PID configured in Epusdt API keys.')}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Epusdt Secret Key')}</Label>
        <Input
          type='password'
          placeholder={t('Leave blank to keep existing secret key')}
          {...form.register('EpusdtSecretKey')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'The secret_key configured in Epusdt API keys, used for request and callback signatures.'
          )}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Fiat Currency')}</Label>
        <Select
          value={form.watch('EpusdtCurrency') || 'cny'}
          onValueChange={(v) => form.setValue('EpusdtCurrency', v ?? 'cny')}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CURRENCIES.map((value) => (
              <SelectItem key={value} value={value}>
                {value.toUpperCase()}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-xs'>
          {t('The currency sent to Epusdt when creating an order.')}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Epusdt Payment Network Types')}</Label>
        <Textarea
          rows={4}
          placeholder={'tron.usdt\nbsc.usdt\nethereum.usdt'}
          {...form.register('EpusdtTradeTypes')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'One local payment method type per line. Use <network>.<default token>; the default token is sent to Epusdt when creating the order.'
          )}
        </p>
      </div>

      <div className='flex justify-end'>
        <Button onClick={handleSave} disabled={loading}>
          {loading ? t('Saving...') : t('Save')}
        </Button>
      </div>
    </div>
  )
}
