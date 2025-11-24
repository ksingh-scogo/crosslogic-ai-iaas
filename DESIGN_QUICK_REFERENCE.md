# CrossLogic Design System - Quick Reference

**One-page visual guide for developers implementing the design system**

---

## Color Palette

```css
/* Primary Brand Colors */
--brand-primary: #0EA5E9;      /* Sky Blue - Main CTAs */
--brand-secondary: #2563EB;    /* Blue - Secondary Actions */
--brand-accent: #8B5CF6;       /* Purple - Highlights */

/* Semantic Colors */
--success: #10B981;  /* Green */
--warning: #F59E0B;  /* Amber */
--error: #EF4444;    /* Red */
--info: #3B82F6;     /* Blue */

/* Neutral Scale (Light Mode) */
--gray-50: #F8FAFC;   --gray-500: #64748B;   --gray-900: #0F172A;
--gray-100: #F1F5F9;  --gray-600: #475569;
--gray-200: #E2E8F0;  --gray-700: #334155;
--gray-300: #CBD5E1;  --gray-800: #1E293B;
--gray-400: #94A3B8;
```

---

## Typography

**Font Families:**
- UI: `Inter, -apple-system, sans-serif`
- Code: `'Fira Code', Menlo, monospace`

**Scale:**
| Element | Size | Weight | Line Height |
|---------|------|--------|-------------|
| H1 | 30px | 700 | 1.2 |
| H2 | 24px | 700 | 1.3 |
| H3 | 20px | 600 | 1.4 |
| Body | 14px | 400 | 1.5 |
| Small | 13px | 400 | 1.5 |
| Caption | 12px | 500 | 1.4 |

---

## Spacing

**4px base unit system:**
```
space-1: 4px    space-6: 24px   space-16: 64px
space-2: 8px    space-8: 32px   space-20: 80px
space-3: 12px   space-10: 40px  space-24: 96px
space-4: 16px   space-12: 48px
space-5: 20px
```

**Common Uses:**
- Component padding: `16-24px`
- Section spacing: `32-48px`
- Element gaps: `8-16px`

---

## Buttons

### Primary
```tsx
<button className="bg-gradient-to-r from-sky-500 to-blue-600
  text-white font-semibold px-5 py-2.5 rounded-lg
  shadow-lg hover:shadow-xl hover:-translate-y-0.5
  transition-all duration-200">
  Launch Instance
</button>
```

### Secondary
```tsx
<button className="bg-white border border-gray-300
  text-gray-700 font-semibold px-5 py-2.5 rounded-lg
  hover:bg-gray-50 hover:border-gray-400
  transition-all duration-200">
  Cancel
</button>
```

### Destructive
```tsx
<button className="bg-red-500 text-white font-semibold
  px-5 py-2.5 rounded-lg hover:bg-red-600
  transition-colors duration-200">
  Terminate
</button>
```

---

## Cards

### Standard Card
```tsx
<div className="bg-white border border-gray-200
  rounded-xl p-5 shadow-sm
  hover:shadow-md transition-shadow duration-200">
  <h3 className="text-lg font-semibold mb-2">Card Title</h3>
  <p className="text-sm text-gray-600">Card content goes here</p>
</div>
```

### Stat Card
```tsx
<div className="bg-gradient-to-b from-white to-gray-50
  border border-gray-200 rounded-2xl p-6 shadow-md">
  <div className="text-xs uppercase tracking-wide text-gray-500 mb-2">
    Total Tokens
  </div>
  <div className="text-3xl font-bold text-gray-900 mb-1">
    1,234,567
  </div>
  <div className="text-xs text-gray-500">
    +20.1% from last month
  </div>
</div>
```

---

## Forms

### Input Field
```tsx
<div className="space-y-1.5">
  <label className="text-sm font-semibold text-gray-700">
    API Key Name
  </label>
  <input
    type="text"
    className="w-full h-10 px-3 border border-gray-300
      rounded-lg text-sm
      focus:outline-none focus:ring-2 focus:ring-blue-500
      focus:border-transparent"
    placeholder="Production API Key"
  />
  <span className="text-xs text-gray-500">
    Used to identify this key in logs
  </span>
</div>
```

### Select Dropdown
```tsx
<select className="w-full h-10 px-3 border border-gray-300
  rounded-lg text-sm appearance-none
  focus:outline-none focus:ring-2 focus:ring-blue-500">
  <option>Select region...</option>
  <option>us-east-1</option>
  <option>us-west-2</option>
</select>
```

---

## Badges & Status

### Status Badge
```tsx
<span className="inline-flex items-center px-2.5 py-1
  rounded-full text-xs font-semibold
  bg-green-100 text-green-700 border border-green-200">
  Running
</span>
```

**Variants:**
- Success: `bg-green-100 text-green-700 border-green-200`
- Warning: `bg-yellow-100 text-yellow-700 border-yellow-200`
- Error: `bg-red-100 text-red-700 border-red-200`
- Info: `bg-blue-100 text-blue-700 border-blue-200`
- Neutral: `bg-gray-100 text-gray-700 border-gray-200`

### Dot Indicator
```tsx
<div className="flex items-center gap-2">
  <div className="w-2 h-2 rounded-full bg-green-500
    animate-pulse" />
  <span className="text-sm text-gray-600">Active</span>
</div>
```

---

## Tables

```tsx
<div className="border border-gray-200 rounded-xl overflow-hidden">
  <table className="w-full">
    <thead className="bg-gray-50 border-b border-gray-200">
      <tr>
        <th className="px-6 py-3 text-left text-xs font-semibold
          uppercase tracking-wider text-gray-600">
          Name
        </th>
        <th className="px-6 py-3 text-left text-xs font-semibold
          uppercase tracking-wider text-gray-600">
          Status
        </th>
      </tr>
    </thead>
    <tbody className="divide-y divide-gray-100">
      <tr className="hover:bg-gray-50 transition-colors">
        <td className="px-6 py-4 text-sm text-gray-900">
          Node-001
        </td>
        <td className="px-6 py-4 text-sm">
          <span className="badge-success">Running</span>
        </td>
      </tr>
    </tbody>
  </table>
</div>
```

---

## Modals

```tsx
<div className="fixed inset-0 z-50 flex items-center justify-center">
  {/* Backdrop */}
  <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" />

  {/* Modal */}
  <div className="relative bg-white rounded-2xl shadow-2xl
    max-w-lg w-full mx-4 p-8">
    <h2 className="text-xl font-bold mb-4">Modal Title</h2>
    <p className="text-sm text-gray-600 mb-6">
      Modal content goes here
    </p>
    <div className="flex justify-end gap-3">
      <button className="btn-secondary">Cancel</button>
      <button className="btn-primary">Confirm</button>
    </div>
  </div>
</div>
```

---

## Loading States

### Spinner
```tsx
<div className="animate-spin rounded-full h-6 w-6
  border-2 border-gray-200 border-t-blue-600" />
```

### Skeleton
```tsx
<div className="animate-pulse space-y-3">
  <div className="h-4 bg-gray-200 rounded w-3/4" />
  <div className="h-4 bg-gray-200 rounded w-1/2" />
</div>
```

### Progress Bar
```tsx
<div className="w-full bg-gray-200 rounded-full h-2">
  <div className="bg-gradient-to-r from-blue-500 to-blue-600
    h-2 rounded-full transition-all duration-500"
    style={{ width: '73%' }} />
</div>
```

---

## Toasts

```tsx
<div className="fixed top-4 right-4 z-50
  bg-white rounded-xl shadow-2xl border border-gray-200
  p-4 min-w-[320px] animate-slide-in-right">
  <div className="flex items-start gap-3">
    <div className="w-5 h-5 rounded-full bg-green-100
      flex items-center justify-center">
      <CheckIcon className="w-3 h-3 text-green-600" />
    </div>
    <div className="flex-1">
      <div className="font-semibold text-sm text-gray-900">
        Success!
      </div>
      <div className="text-sm text-gray-600">
        API key created successfully
      </div>
    </div>
    <button className="text-gray-400 hover:text-gray-600">
      <XIcon className="w-4 h-4" />
    </button>
  </div>
</div>
```

---

## Responsive Breakpoints

```css
/* Mobile: < 768px */
@media (max-width: 767px) {
  /* Stack vertically, hamburger menu */
}

/* Tablet: 768px - 1024px */
@media (min-width: 768px) and (max-width: 1023px) {
  /* 2-column layouts, icon-only sidebar */
}

/* Desktop: >= 1024px */
@media (min-width: 1024px) {
  /* Full layouts, expanded sidebar */
}
```

---

## Accessibility

**Minimum Requirements:**
- Color contrast: 4.5:1 for text, 3:1 for UI
- Focus indicators: 2px blue outline
- Keyboard navigation: Tab through all interactive elements
- ARIA labels: For icon-only buttons
- Alt text: For all images

**Example:**
```tsx
<button
  aria-label="Close modal"
  className="focus:outline-none focus:ring-2
    focus:ring-blue-500 focus:ring-offset-2">
  <XIcon className="w-5 h-5" />
</button>
```

---

## Animation Timing

```css
/* Fast (hover, clicks) */
transition-all duration-150

/* Medium (modals, dropdowns) */
transition-all duration-300

/* Slow (page transitions) */
transition-all duration-500
```

**Easing:**
- Enter: `ease-out`
- Exit: `ease-in`
- State change: `ease-in-out`

---

## Common Patterns

### Empty State
```tsx
<div className="text-center py-16">
  <div className="w-16 h-16 rounded-full bg-gray-100
    flex items-center justify-center mx-auto mb-4">
    <ServerIcon className="w-8 h-8 text-gray-400" />
  </div>
  <h3 className="text-lg font-semibold text-gray-900 mb-2">
    No active nodes
  </h3>
  <p className="text-sm text-gray-500 mb-6">
    Launch your first instance to get started
  </p>
  <button className="btn-primary">
    Launch Instance
  </button>
</div>
```

### Error State
```tsx
<div className="rounded-lg border-l-4 border-red-500
  bg-red-50 p-4">
  <div className="flex items-start">
    <AlertCircleIcon className="w-5 h-5 text-red-500 mt-0.5" />
    <div className="ml-3">
      <h3 className="text-sm font-semibold text-red-800">
        Error
      </h3>
      <p className="text-sm text-red-700 mt-1">
        Failed to create API key. Name must be at least 3 characters.
      </p>
      <button className="text-sm font-semibold text-red-700
        underline mt-2">
        Try again
      </button>
    </div>
  </div>
</div>
```

---

## Z-Index Scale

```css
--z-base: 0;
--z-dropdown: 1000;
--z-sticky: 1020;
--z-modal-backdrop: 1040;
--z-modal: 1050;
--z-tooltip: 1070;
```

---

## Implementation Checklist

**Before starting:**
- [ ] Review full design spec (`DESIGN_SPECIFICATION.md`)
- [ ] Install dependencies: `inter` font, `lucide-react` icons
- [ ] Set up Tailwind with custom colors
- [ ] Create base component structure

**Per component:**
- [ ] Match exact spacing from design
- [ ] Test all states (hover, active, disabled, error)
- [ ] Verify responsive behavior
- [ ] Test keyboard navigation
- [ ] Check color contrast
- [ ] Add ARIA labels
- [ ] Test dark mode (Phase 2)

**Per page:**
- [ ] Desktop layout matches Figma
- [ ] Tablet view works correctly
- [ ] Mobile view tested on device
- [ ] Loading states implemented
- [ ] Error handling in place
- [ ] Empty states shown when appropriate

---

## Quick Tips

1. **Use consistent spacing**: Always use the 4px grid (space-1, space-2, etc.)
2. **Border radius**: 8px for buttons/inputs, 12px for cards, 16px for large cards
3. **Shadows**: Subtle by default, increase on hover
4. **Icons**: Always 16px or 20px, use Lucide icons
5. **Transitions**: 200ms for most interactions
6. **Focus states**: Never remove outlines, style them instead
7. **Touch targets**: Minimum 44x44px on mobile
8. **Text truncation**: Use `truncate` class for long strings
9. **Loading states**: Show skeletons for predictable content, spinners otherwise
10. **Error messages**: Be specific, suggest solutions

---

## Resources

**Documentation:**
- Full Design Spec: `DESIGN_SPECIFICATION.md`
- Figma Library: [Link to Figma]
- Component Storybook: [Link to Storybook]

**Tools:**
- Icons: https://lucide.dev
- Colors: https://tailwindcss.com/docs/customizing-colors
- Fonts: Inter (Google Fonts), Fira Code

**Testing:**
- Color Contrast: https://webaim.org/resources/contrastchecker/
- Accessibility: axe DevTools (browser extension)
- Responsive: Chrome DevTools device mode

---

**Quick Reference Version:** 1.0
**Last Updated:** November 24, 2025
**For questions:** Refer to full design spec or contact design team
