/**
 * 自绘下拉（listbox）。
 *
 * 为什么不用原生 <select>：
 * 原生下拉由操作系统绘制 —— 半透明圆角、系统字号、系统高亮色，
 * 一样都改不了。在浅色纸面里它像块贴上去的异物，也不跟随主题。
 *
 * 这里保证：
 *   触发器与输入框同规格（同高、同描边、同聚焦环）；
 *   菜单是应用自己的面板，有浮起阴影；
 *   键盘 ↑↓ Home End Enter Esc Tab 全通，焦点不会丢；
 *   点外面、窗口失焦都关闭。
 */
;(function (global) {
  'use strict'

  var seq = 0
  var current = null // 当前打开的下拉，全局只允许一个

  function Chevron() {
    return (
      '<svg class="select-arrow h-3.5 w-3.5" viewBox="0 0 16 16" fill="none" aria-hidden="true">' +
      '<path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" ' +
      'stroke-linecap="round" stroke-linejoin="round"/></svg>'
    )
  }

  function Select(root, config) {
    config = config || {}
    this.root = root
    this.placeholder = config.placeholder || '请选择'
    this.onChange = config.onChange || function () {}
    this.items = []
    this.value = config.value === undefined ? '' : config.value
    this.id = 'select-' + ++seq
    this.build()
    this.setItems(config.items || [], this.value)
  }

  Select.prototype.build = function () {
    var root = this.root
    root.classList.add('select')
    root.setAttribute('role', 'combobox')
    root.setAttribute('tabindex', '0')
    root.setAttribute('aria-haspopup', 'listbox')
    root.setAttribute('aria-expanded', 'false')
    root.innerHTML =
      '<span class="select-value"></span>' + Chevron()

    var self = this
    root.addEventListener('click', function (event) {
      event.preventDefault()
      self.toggle()
    })
    root.addEventListener('keydown', function (event) {
      self.onKeydown(event)
    })
    root.addEventListener('blur', function (event) {
      // 焦点真的离开这个下拉（而不是移到它自己的菜单里）才关闭
      if (!root.contains(event.relatedTarget)) self.close()
    })
  }

  Select.prototype.setItems = function (items, value) {
    this.items = items || []
    if (value !== undefined && value !== null) this.value = value
    if (!this.hasValue() && this.items.length) this.value = this.items[0].value
    this.paint()
  }

  Select.prototype.hasValue = function () {
    for (var i = 0; i < this.items.length; i++) {
      if (this.items[i].value === this.value) return true
    }
    return false
  }

  Select.prototype.paint = function () {
    var span = this.root.querySelector('.select-value')
    var item = this.itemByValue(this.value)
    if (item) {
      span.textContent = item.label
      span.classList.remove('is-placeholder')
      this.root.setAttribute('aria-activedescendant', '')
    } else {
      span.textContent = this.placeholder
      span.classList.add('is-placeholder')
    }
  }

  Select.prototype.itemByValue = function (value) {
    for (var i = 0; i < this.items.length; i++) {
      if (this.items[i].value === value) return this.items[i]
    }
    return null
  }

  Select.prototype.toggle = function () {
    if (this.root.getAttribute('aria-expanded') === 'true') this.close()
    else this.open()
  }

  Select.prototype.open = function () {
    if (!this.items.length) return
    if (current && current !== this) current.close()
    current = this

    var menu = document.createElement('div')
    menu.className = 'select-menu'
    menu.setAttribute('role', 'listbox')
    menu.id = this.id + '-listbox'
    this.root.setAttribute('aria-controls', menu.id)
    this.root.setAttribute('aria-expanded', 'true')

    var self = this
    this.items.forEach(function (item, index) {
      var option = document.createElement('div')
      option.className = 'select-option'
      option.setAttribute('role', 'option')
      option.id = self.id + '-opt-' + index
      option.setAttribute('aria-selected', item.value === self.value ? 'true' : 'false')
      option.innerHTML =
        '<span class="select-tick"></span><span class="min-w-0 truncate">' +
        escapeHTML(item.label) +
        '</span>' +
        (item.note ? '<span class="ml-auto shrink-0 text-2xs text-ink-dim">' + escapeHTML(item.note) + '</span>' : '')
      option.addEventListener('click', function (event) {
        event.stopPropagation()
        self.pick(item.value)
      })
      option.addEventListener('mousemove', function () {
        self.setActive(index)
      })
      menu.appendChild(option)
    })

    this.root.appendChild(menu)
    this.menu = menu
    this.activeIndex = this.indexOfValue(this.value)
    this.setActive(this.activeIndex, true)

    // 点外面关闭
    this._onDocDown = function (event) {
      if (!self.root.contains(event.target)) self.close()
    }
    document.addEventListener('mousedown', this._onDocDown, true)
  }

  Select.prototype.close = function () {
    if (!this.menu) return
    this.menu.remove()
    this.menu = null
    this.root.setAttribute('aria-expanded', 'false')
    this.root.removeAttribute('aria-activedescendant')
    document.removeEventListener('mousedown', this._onDocDown, true)
    if (current === this) current = null
  }

  Select.prototype.indexOfValue = function (value) {
    for (var i = 0; i < this.items.length; i++) {
      if (this.items[i].value === value) return i
    }
    return -1
  }

  Select.prototype.setActive = function (index, scroll) {
    if (!this.menu) return
    var options = this.menu.querySelectorAll('.select-option')
    for (var i = 0; i < options.length; i++) {
      options[i].classList.remove('is-active')
    }
    if (index < 0 || index >= options.length) return
    this.activeIndex = index
    options[index].classList.add('is-active')
    this.root.setAttribute('aria-activedescendant', options[index].id)
    if (scroll) {
      options[index].scrollIntoView({ block: 'nearest' })
    }
  }

  Select.prototype.pick = function (value) {
    var changed = value !== this.value
    this.value = value
    this.paint()
    this.close()
    this.root.focus()
    if (changed) this.onChange(value)
  }

  Select.prototype.onKeydown = function (event) {
    var open = this.root.getAttribute('aria-expanded') === 'true'

    if (!open) {
      if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault()
        this.open()
      }
      return
    }

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        this.setActive(Math.min(this.activeIndex + 1, this.items.length - 1), true)
        break
      case 'ArrowUp':
        event.preventDefault()
        this.setActive(Math.max(this.activeIndex - 1, 0), true)
        break
      case 'Home':
        event.preventDefault()
        this.setActive(0, true)
        break
      case 'End':
        event.preventDefault()
        this.setActive(this.items.length - 1, true)
        break
      case 'Enter':
      case ' ':
        event.preventDefault()
        if (this.activeIndex >= 0) this.pick(this.items[this.activeIndex].value)
        break
      case 'Escape':
        event.preventDefault()
        this.close()
        this.root.focus()
        break
      case 'Tab':
        this.close()
        break
      default:
        break
    }
  }

  function escapeHTML(text) {
    return String(text)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
  }

  global.ReelSelect = {
    create: function (root, config) {
      return new Select(root, config)
    },
  }
})(window)
