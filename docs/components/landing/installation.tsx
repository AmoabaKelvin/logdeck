"use client"

import { motion } from "framer-motion"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { CodeBlock } from "./code-block"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import Link from "next/link"

const dockerComposeExample = `services:
  logdeck:
    image: amoabakelvin/logdeck:latest
    container_name: logdeck
    ports:
      - "8123:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /proc:/host/proc:ro
    environment:
      # Optional: Enable authentication
      # JWT_SECRET: your-super-secret-key-min-32-chars
      # ADMIN_USERNAME: admin
      # ADMIN_PASSWORD_SALT: your-random-salt-change-this
      # ADMIN_PASSWORD: your-sha256-hash
    restart: unless-stopped`

const dockerRunCommand = `docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  amoabakelvin/logdeck:latest`

const dockerRunWithAuth = `docker run -d \\
  --name logdeck \\
  -p 8123:8080 \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v /proc:/host/proc:ro \\
  -e JWT_SECRET=your-super-secret-key-min-32-chars \\
  -e ADMIN_USERNAME=admin \\
  -e ADMIN_PASSWORD_SALT=your-random-salt-change-this \\
  -e ADMIN_PASSWORD=your-sha256-hash \\
  amoabakelvin/logdeck:latest`

export function Installation() {
  return (
    <section id="installation" className="container py-20 md:py-32">
      <div className="mx-auto flex max-w-[58rem] flex-col items-center space-y-4 text-center mb-16">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5 }}
        >
          <h2 className="text-3xl font-bold leading-[1.1] sm:text-4xl md:text-5xl">
            Get started in seconds
          </h2>
          <p className="mt-4 text-lg text-muted-foreground sm:text-xl">
            Choose your preferred installation method and start monitoring your containers
          </p>
        </motion.div>
      </div>

      <motion.div
        initial={{ opacity: 0, y: 40 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.6 }}
        className="mx-auto max-w-5xl"
      >
        <Tabs defaultValue="compose" className="w-full">
          <TabsList className="grid w-full max-w-md mx-auto grid-cols-2">
            <TabsTrigger value="compose">Docker Compose</TabsTrigger>
            <TabsTrigger value="docker">Docker Run</TabsTrigger>
          </TabsList>

          <TabsContent value="compose" className="mt-8">
            <Card>
              <CardHeader>
                <CardTitle>Docker Compose (Recommended)</CardTitle>
                <CardDescription>
                  The easiest way to deploy LogDeck with persistent configuration
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-3">
                  <p className="text-sm font-medium">
                    1. Create a <code className="text-xs bg-muted px-1.5 py-0.5 rounded">docker-compose.yml</code> file:
                  </p>
                  <CodeBlock code={dockerComposeExample} language="yaml" />
                </div>

                <div className="space-y-3">
                  <p className="text-sm font-medium">2. Start LogDeck:</p>
                  <CodeBlock code="docker-compose up -d" language="bash" />
                </div>

                <div className="space-y-3">
                  <p className="text-sm font-medium">3. Access the interface:</p>
                  <div className="rounded-lg border bg-muted/50 p-4">
                    <p className="text-sm">
                      Open your browser and navigate to{" "}
                      <a
                        href="http://localhost:8123"
                        className="font-mono text-primary hover:underline"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        http://localhost:8123
                      </a>
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="docker" className="mt-8">
            <Card>
              <CardHeader>
                <CardTitle>Docker Run</CardTitle>
                <CardDescription>
                  Quick deployment using the docker run command
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-3">
                  <p className="text-sm font-medium">Basic deployment:</p>
                  <CodeBlock code={dockerRunCommand} language="bash" />
                </div>

                <div className="space-y-3">
                  <p className="text-sm font-medium">With authentication (optional):</p>
                  <CodeBlock code={dockerRunWithAuth} language="bash" />
                </div>

                <div className="rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/20 p-4">
                  <p className="text-sm text-amber-900 dark:text-amber-200">
                    <strong>Note:</strong> For authentication, you&apos;ll need to generate a random salt and SHA256 hash for your password.
                    See the{" "}
                    <Link href="/docs/configuration" className="underline hover:no-underline">
                      configuration guide
                    </Link>{" "}
                    for details.
                  </p>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        <div className="mt-12 text-center">
          <p className="text-sm text-muted-foreground mb-4">
            Need more details on configuration and advanced setup?
          </p>
          <Button asChild variant="outline">
            <Link href="/docs/installation" className="gap-2">
              View Full Documentation
              <ArrowRight className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </motion.div>
    </section>
  )
}
