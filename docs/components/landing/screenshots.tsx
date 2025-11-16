"use client"

import { motion } from "framer-motion"
import Image from "next/image"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

export function Screenshots() {
  return (
    <section className="border-y bg-muted/30 py-20 md:py-32">
      <div className="container">
        <div className="mx-auto flex max-w-[58rem] flex-col items-center space-y-4 text-center mb-16">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5 }}
          >
            <h2 className="text-3xl font-bold leading-[1.1] sm:text-4xl md:text-5xl">
              Beautiful interface, powerful features
            </h2>
            <p className="mt-4 text-lg text-muted-foreground sm:text-xl">
              A modern, intuitive UI that makes Docker management a breeze
            </p>
          </motion.div>
        </div>

        <motion.div
          initial={{ opacity: 0, y: 40 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.6 }}
        >
          <Tabs defaultValue="dashboard" className="w-full">
            <TabsList className="grid w-full max-w-md mx-auto grid-cols-2">
              <TabsTrigger value="dashboard">Container Dashboard</TabsTrigger>
              <TabsTrigger value="logs">Log Viewer</TabsTrigger>
            </TabsList>
            <TabsContent value="dashboard" className="mt-8">
              <div className="relative mx-auto max-w-6xl overflow-hidden rounded-xl border bg-background shadow-2xl">
                <Image
                  src="/landing.png"
                  alt="LogDeck Container Dashboard - View all your containers with status, uptime, and management controls"
                  width={1920}
                  height={1080}
                  className="w-full h-auto"
                  priority
                />
              </div>
              <div className="mt-6 text-center">
                <p className="text-sm text-muted-foreground">
                  Monitor all your containers at a glance with real-time status updates, system metrics, and quick actions
                </p>
              </div>
            </TabsContent>
            <TabsContent value="logs" className="mt-8">
              <div className="relative mx-auto max-w-6xl overflow-hidden rounded-xl border bg-background shadow-2xl">
                <Image
                  src="/logs.png"
                  alt="LogDeck Log Viewer - Real-time container logs with filtering, search, and download capabilities"
                  width={1920}
                  height={1080}
                  className="w-full h-auto"
                />
              </div>
              <div className="mt-6 text-center">
                <p className="text-sm text-muted-foreground">
                  Stream logs in real-time with advanced filtering, search, and color-coded log levels
                </p>
              </div>
            </TabsContent>
          </Tabs>
        </motion.div>
      </div>
    </section>
  )
}
