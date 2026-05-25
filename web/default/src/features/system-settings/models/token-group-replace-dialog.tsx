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
import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, ArrowRight, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { countTokensByGroup, replaceTokenGroup } from '../api'

type TokenGroupReplaceDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  availableGroups: string[]
}

type RequestState = 'idle' | 'counting' | 'replacing'

export function TokenGroupReplaceDialog({
  open,
  onOpenChange,
  availableGroups,
}: TokenGroupReplaceDialogProps) {
  const { t } = useTranslation()
  const [sourceGroup, setSourceGroup] = useState('')
  const [targetGroup, setTargetGroup] = useState('')
  const [matchedCount, setMatchedCount] = useState<number | null>(null)
  const [requestState, setRequestState] = useState<RequestState>('idle')

  const trimmedSource = sourceGroup.trim()
  const trimmedTarget = targetGroup.trim()
  const isSameGroup = trimmedSource !== '' && trimmedSource === trimmedTarget
  const canCount = trimmedSource !== '' && requestState === 'idle'
  const canReplace =
    trimmedSource !== '' &&
    trimmedTarget !== '' &&
    !isSameGroup &&
    matchedCount !== null &&
    matchedCount > 0 &&
    requestState === 'idle'

  const sortedGroups = useMemo(
    () => [...availableGroups].sort((a, b) => a.localeCompare(b)),
    [availableGroups]
  )

  useEffect(() => {
    if (!open) {
      setSourceGroup('')
      setTargetGroup('')
      setMatchedCount(null)
      setRequestState('idle')
    }
  }, [open])

  useEffect(() => {
    setMatchedCount(null)
  }, [trimmedSource])

  const handleCount = async () => {
    if (!trimmedSource) return
    setRequestState('counting')
    try {
      const result = await countTokensByGroup(trimmedSource)
      if (result.success) {
        setMatchedCount(result.data?.count ?? 0)
      } else {
        toast.error(result.message || t('Failed to count matching tokens'))
      }
    } catch {
      toast.error(t('Failed to count matching tokens'))
    } finally {
      setRequestState('idle')
    }
  }

  const handleReplace = async () => {
    if (!canReplace) return
    setRequestState('replacing')
    try {
      const result = await replaceTokenGroup({
        source_group: trimmedSource,
        target_group: trimmedTarget,
      })
      if (result.success) {
        const count = result.data?.count ?? 0
        toast.success(
          t('Successfully replaced group for {{count}} token(s)', { count })
        )
        onOpenChange(false)
      } else {
        toast.error(result.message || t('Failed to replace token groups'))
      }
    } catch {
      toast.error(t('Failed to replace token groups'))
    } finally {
      setRequestState('idle')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Batch replace token groups')}</DialogTitle>
          <DialogDescription>
            {t(
              'Find tokens using an old group name and update them to a new group name.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <Alert>
            <AlertTriangle className='size-4' />
            <AlertTitle>{t('Affects all users')}</AlertTitle>
            <AlertDescription>
              {t(
                'This only changes saved token group names. It does not rename group ratio settings or channel groups.'
              )}
            </AlertDescription>
          </Alert>

          <div className='grid gap-3 sm:grid-cols-[1fr_auto_1fr] sm:items-end'>
            <div className='space-y-2'>
              <Label htmlFor='source-token-group'>{t('Old token group')}</Label>
              <Input
                id='source-token-group'
                value={sourceGroup}
                onChange={(event) => setSourceGroup(event.target.value)}
                placeholder={t('Enter old group name')}
                list='token-group-replace-options'
              />
            </div>
            <ArrowRight className='text-muted-foreground mb-2 hidden size-4 sm:block' />
            <div className='space-y-2'>
              <Label htmlFor='target-token-group'>{t('New token group')}</Label>
              <Input
                id='target-token-group'
                value={targetGroup}
                onChange={(event) => setTargetGroup(event.target.value)}
                placeholder={t('Enter new group name')}
                list='token-group-replace-options'
              />
            </div>
          </div>

          <datalist id='token-group-replace-options'>
            {sortedGroups.map((group) => (
              <option key={group} value={group} />
            ))}
          </datalist>

          {isSameGroup && (
            <p className='text-destructive text-sm'>
              {t('Old and new groups must be different.')}
            </p>
          )}

          {matchedCount !== null && (
            <div className='bg-muted/50 rounded-lg border px-3 py-2 text-sm'>
              {t('Matched {{count}} token(s) in group "{{group}}"', {
                count: matchedCount,
                group: trimmedSource,
              })}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={requestState !== 'idle'}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            variant='outline'
            onClick={handleCount}
            disabled={!canCount}
          >
            {requestState === 'counting' && (
              <Loader2 className='size-4 animate-spin' />
            )}
            {t('Count tokens')}
          </Button>
          <Button type='button' onClick={handleReplace} disabled={!canReplace}>
            {requestState === 'replacing' && (
              <Loader2 className='size-4 animate-spin' />
            )}
            {t('Replace groups')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
