import { useState, useCallback, type DragEvent } from 'react'
import { toast } from 'sonner'
import { Upload } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface Props {
  onFile: (file: File) => void
  loading?: boolean
  accept?: string
}

export default function WasmUploader({ onFile, loading, accept = '.wasm,.zip' }: Props) {
  const [dragOver, setDragOver] = useState(false)

  const validateAndSubmit = useCallback(
    (file: File | undefined) => {
      if (!file) return
      const lowerName = file.name.toLowerCase()
      if (!lowerName.endsWith('.wasm') && !lowerName.endsWith('.zip')) {
        toast.error('Поддерживаются только .wasm и .zip файлы')
        return
      }
      onFile(file)
    },
    [onFile],
  )

  const handleDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      validateAndSubmit(e.dataTransfer.files[0])
    },
    [validateAndSubmit],
  )

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    validateAndSubmit(e.target.files?.[0])
    e.target.value = ''
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault()
        if (!loading) setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
      className={cn(
        'border-2 border-dashed rounded-xl p-8 sm:p-12 text-center transition-all duration-200',
        dragOver
          ? 'border-primary bg-primary/5 ring-4 ring-primary/20 shadow-lg'
          : 'border-muted-foreground/25 bg-muted/30 hover:border-muted-foreground/40',
        loading && 'opacity-50 pointer-events-none',
      )}
    >
      <div className="text-muted-foreground mb-3">
        <Upload
          className={cn(
            'mx-auto h-10 w-10 transition-transform duration-200',
            dragOver && 'scale-125 text-primary',
          )}
          strokeWidth={1.5}
        />
      </div>
      <p className="text-muted-foreground mb-1 text-sm sm:text-base">
        {loading ? 'Загрузка...' : 'Перетащите .wasm или .zip файл сюда'}
      </p>
      <p className="text-xs text-muted-foreground/60 mb-4">Максимальный размер: 50 МБ. ZIP может содержать frontend/</p>
      <Button asChild size="sm">
        <label className="cursor-pointer">
          Выбрать файл
          <input type="file" accept={accept} onChange={handleChange} className="hidden" />
        </label>
      </Button>
    </div>
  )
}
