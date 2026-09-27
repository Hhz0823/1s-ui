import { computed, ref, watch, type Ref } from 'vue'
import { useDisplay } from 'vuetify'
import { i18n } from '@/locales'

const pageSizes = [20, 40, 80]

// Shared state of the toolbar + table list pages: page size (optionally kept
// per page) and the stacked layout on phones.
export const useListTable = (storageKey?: string) => {
  const { smAndDown } = useDisplay()
  const saved = storageKey ? Number(localStorage.getItem(storageKey)) : Number.NaN
  const itemsPerPage = ref([...pageSizes, -1].includes(saved) ? saved : 20)
  if (storageKey) {
    watch(itemsPerPage, value => localStorage.setItem(storageKey, String(value)))
  }
  const pageSizeItems = computed(() => [
    ...pageSizes.map(value => ({ value, title: String(value) })),
    { value: -1, title: i18n.global.t('list.all') },
  ])
  return { smAndDown, itemsPerPage, pageSizeItems }
}

// matchesQuery reports whether any field contains the search text.
export const matchesQuery = (query: string | null | undefined, ...fields: unknown[]) => {
  const value = String(query ?? '').trim().toLocaleLowerCase()
  if (!value) return true
  return fields.some(field => String(field ?? '').toLocaleLowerCase().includes(value))
}

export const hostPort = (host: unknown, port: unknown) => {
  if (!host && !port) return '-'
  const text = String(host ?? '')
  return (text.includes(':') ? `[${text}]` : text) + (port ? `:${port}` : '')
}

// useRowReorder drags table rows onto each other to reorder the list behind
// them. Each row carries its list index, so filtering keeps positions right.
// `move` also backs the arrow buttons, since phones cannot drag.
export const useRowReorder = (list: () => unknown[], disabled: Ref<boolean>) => {
  const dragged = ref<number | null>(null)
  const target = ref<number | null>(null)
  const move = (from: number, to: number) => {
    const items = list()
    if (from == to || to < 0 || to >= items.length) return
    const [moved] = items.splice(from, 1)
    items.splice(to, 0, moved)
  }
  const endDrag = () => {
    dragged.value = null
    target.value = null
  }
  const rowProps = ({ item }: { item: { index: number } }) => disabled.value ? {} : {
    // The moved row lands before a target above it and after one below it.
    class: {
      'list-row--dragging': dragged.value == item.index,
      'list-row--drop-before': target.value == item.index && dragged.value != null && dragged.value > item.index,
      'list-row--drop-after': target.value == item.index && dragged.value != null && dragged.value < item.index,
    },
    draggable: true,
    onDragstart: (event: DragEvent) => {
      dragged.value = item.index
      // Firefox only starts a drag that carries data.
      event.dataTransfer?.setData('text/plain', String(item.index + 1))
      if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
    },
    onDragover: (event: DragEvent) => {
      if (dragged.value == null) return
      event.preventDefault()
      target.value = item.index
    },
    onDrop: (event: DragEvent) => {
      event.preventDefault()
      if (dragged.value != null) move(dragged.value, item.index)
      endDrag()
    },
    onDragend: endDrag,
  }
  return { move, rowProps }
}
