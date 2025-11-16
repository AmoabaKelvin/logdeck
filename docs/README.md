# LogDeck Documentation & Landing Page

This is the official documentation and landing page for LogDeck, built with Next.js 16, Tailwind CSS v4, and shadcn/ui.

## 🚀 Quick Start

```bash
# Install dependencies
bun install

# Run development server
bun run dev

# Build for production
bun run build

# Start production server
bun run start
```

The site will be available at [http://localhost:3000](http://localhost:3000)

## 📁 Project Structure

```
docs/
├── app/                      # Next.js app directory
│   ├── layout.tsx           # Root layout with metadata
│   ├── page.tsx             # Landing page
│   ├── globals.css          # Global styles and Tailwind config
│   └── docs/                # Documentation pages
│       ├── layout.tsx       # Docs layout with sidebar
│       ├── getting-started/
│       ├── installation/
│       ├── features/
│       └── configuration/
├── components/
│   ├── landing/             # Landing page components
│   │   ├── hero.tsx        # Hero section with GitHub stars
│   │   ├── features.tsx    # Feature cards
│   │   ├── screenshots.tsx # Screenshot showcase
│   │   ├── installation.tsx # Installation guide
│   │   └── code-block.tsx  # Code block with copy
│   ├── docs/               # Documentation components
│   │   └── sidebar.tsx     # Docs sidebar navigation
│   ├── ui/                 # shadcn/ui components
│   ├── navbar.tsx          # Main navigation
│   ├── footer.tsx          # Site footer
│   ├── theme-provider.tsx  # Theme provider
│   └── theme-toggle.tsx    # Dark/light mode toggle
├── lib/
│   └── utils.ts            # Utility functions
└── public/                 # Static assets
    ├── landing.png         # Landing page screenshot
    └── logs.png            # Logs viewer screenshot
```

## 🎨 Features

- **Modern Landing Page**
  - Animated hero section with Framer Motion
  - GitHub stars integration
  - Feature cards with icons
  - Screenshot showcase with tabs
  - Installation guide with code blocks

- **Documentation Site**
  - Sidebar navigation
  - Multiple documentation pages
  - Code blocks with copy-to-clipboard
  - Responsive design

- **SEO Optimized**
  - Comprehensive metadata
  - Open Graph tags
  - Twitter cards
  - Structured data

- **UI/UX**
  - Dark/light mode support
  - Responsive design
  - Smooth animations
  - Accessible components

## 🛠️ Tech Stack

- **Framework:** Next.js 16.0.3
- **Styling:** Tailwind CSS v4
- **UI Components:** shadcn/ui
- **Icons:** Lucide React
- **Animations:** Framer Motion
- **Code Quality:** Biome
- **Package Manager:** Bun

## 📝 Adding New Documentation

To add a new documentation page:

1. Create a new folder in `app/docs/`
2. Add a `page.tsx` file with your content
3. Update the sidebar navigation in `components/docs/sidebar.tsx`

Example:

```tsx
// app/docs/your-page/page.tsx
import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "Your Page Title",
  description: "Your page description",
}

export default function YourPage() {
  return (
    <div className="space-y-6">
      {/* Your content */}
    </div>
  )
}
```

## 🎨 Customization

### Colors

The color scheme is defined in `app/globals.css` using CSS variables with OKLCH color space:

```css
@theme {
  --color-primary: oklch(var(--primary));
  /* ... other colors */
}
```

### Components

shadcn/ui components can be added using:

```bash
bunx shadcn@latest add [component-name]
```

## 🚢 Deployment

The site is ready to deploy on Vercel, Netlify, or any platform that supports Next.js.

### Vercel

```bash
vercel deploy
```

### Build Locally

```bash
bun run build
bun run start
```

## 📄 License

This documentation site is part of the LogDeck project, licensed under GPL-3.0.
