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
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export interface BEPUsdtSettingsValues {
  BEPUsdtEnabled: boolean
  BEPUsdtApiUrl: string
  BEPUsdtToken: string
  BEPUsdtFiatCurrency: string
  BEPUsdtTradeType: string
  BEPUsdtTradeTypes: string
}

interface Props {
  defaultValues: BEPUsdtSettingsValues
}

const FIAT_CURRENCIES = ['CNY', 'USD', 'EUR', 'GBP', 'JPY']

function parseTradeTypes(value: string): string[] {
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

export function BEPUsdtSettingsSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [loading, setLoading] = useState(false)
  const form = useForm<BEPUsdtSettingsValues>({
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [props.defaultValues, form])

  const handleSave = async () => {
    const values = form.getValues()
    const enabled = !!values.BEPUsdtEnabled

    if (enabled && !values.BEPUsdtApiUrl.trim()) {
      toast.error(t('BEPUsdt API URL is required'))
      return
    }
    if (enabled && !/^https?:\/\//.test(values.BEPUsdtApiUrl.trim())) {
      toast.error(t('BEPUsdt API URL must start with http:// or https://'))
      return
    }
    const fiatCurrency = FIAT_CURRENCIES.includes(values.BEPUsdtFiatCurrency)
      ? values.BEPUsdtFiatCurrency
      : 'CNY'
    const tradeTypes = parseTradeTypes(values.BEPUsdtTradeTypes)
    if (enabled && tradeTypes.length === 0) {
      toast.error(t('Required'))
      return
    }
    const defaultTradeType =
      values.BEPUsdtTradeType.trim() || tradeTypes[0] || 'usdt.bep20'
    if (defaultTradeType && !tradeTypes.includes(defaultTradeType)) {
      tradeTypes.unshift(defaultTradeType)
    }

    setLoading(true)
    try {
      const options: { key: string; value: string }[] = [
        { key: 'BEPUsdtEnabled', value: enabled ? 'true' : 'false' },
        {
          key: 'BEPUsdtApiUrl',
          value: values.BEPUsdtApiUrl.trim().replace(/\/$/, ''),
        },
        {
          key: 'BEPUsdtTradeType',
          value: defaultTradeType,
        },
        {
          key: 'BEPUsdtFiatCurrency',
          value: fiatCurrency,
        },
        {
          key: 'BEPUsdtTradeTypes',
          value: tradeTypes.join('\n'),
        },
      ]

      // Only send token if non-empty (avoid clearing existing value with blank)
      if (values.BEPUsdtToken.trim()) {
        options.push({ key: 'BEPUsdtToken', value: values.BEPUsdtToken.trim() })
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

  const configuredTradeTypes = parseTradeTypes(
    form.watch('BEPUsdtTradeTypes') || ''
  )
  const selectedTradeType = form.watch('BEPUsdtTradeType')
  const selectableTradeTypes =
    selectedTradeType && !configuredTradeTypes.includes(selectedTradeType)
      ? [selectedTradeType, ...configuredTradeTypes]
      : configuredTradeTypes
  const selectOptions =
    selectableTradeTypes.length > 0 ? selectableTradeTypes : ['usdt.bep20']

  return (
    <SettingsSection
      title={t('BEPUsdt Payment Gateway')}
      description={t(
        'Configure BEPUsdt (Epusdt-compatible) USDT crypto payment gateway'
      )}
    >
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'BEPUsdt is an Epusdt-compatible personal crypto payment gateway. Deploy BEPUsdt (https://github.com/v03413/bepusdt) and fill in the API URL and Token below. Callback URL: <ServerAddress>/api/user/bepusdt/notify'
          )}
        </AlertDescription>
      </Alert>

      <div className='flex items-center gap-2'>
        <Switch
          checked={form.watch('BEPUsdtEnabled')}
          onCheckedChange={(v) => form.setValue('BEPUsdtEnabled', v)}
        />
        <Label>{t('Enable BEPUsdt')}</Label>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('BEPUsdt Service URL')}</Label>
        <Input
          placeholder='https://bepusdt.example.com'
          {...form.register('BEPUsdtApiUrl')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'The base URL of your BEPUsdt service (without trailing slash). The order creation endpoint /api/v1/order/create-transaction will be appended automatically.'
          )}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('API Token')}</Label>
        <Input
          type='password'
          placeholder={t('Leave blank to keep existing token')}
          {...form.register('BEPUsdtToken')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'The API authentication token set in BEPUsdt (corresponds to the AUTH_TOKEN environment variable).'
          )}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Fiat Currency')}</Label>
        <Select
          value={form.watch('BEPUsdtFiatCurrency') || 'CNY'}
          onValueChange={(v) => form.setValue('BEPUsdtFiatCurrency', v)}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FIAT_CURRENCIES.map((value) => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-xs'>
          {t('The fiat currency sent to BEPUsdt when creating an order.')}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Default Trade Type')}</Label>
        <Select
          value={selectedTradeType || configuredTradeTypes[0] || 'usdt.bep20'}
          onValueChange={(v) => form.setValue('BEPUsdtTradeType', v)}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {selectOptions.map((value) => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Used when BEPUsdt is enabled but no specific BEPUsdt payment method is configured.'
          )}
        </p>
      </div>

      <div className='grid gap-1.5'>
        <Label>{t('Supported Trade Types')}</Label>
        <Textarea
          rows={4}
          placeholder={'usdt.bep20\nusdt.aptos\nusdt.arbitrum'}
          {...form.register('BEPUsdtTradeTypes')}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'One type per line. A payment method whose type matches this list will use BEPUsdt checkout.'
          )}
        </p>
      </div>

      <div className='flex justify-end'>
        <Button onClick={handleSave} disabled={loading}>
          {loading ? t('Saving...') : t('Save')}
        </Button>
      </div>
    </SettingsSection>
  )
}
