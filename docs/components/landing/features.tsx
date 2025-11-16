"use client";

import { motion } from "framer-motion";
import { Container, Download, Eye, Layers, Shield, Zap } from "lucide-react";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const features = [
  {
    icon: Eye,
    title: "Real-time Log Streaming",
    description:
      "Monitor your container logs in real-time with auto-scroll, timestamps, and color-coded log levels. Search, filter, and download logs with ease.",
    number: "01",
  },
  {
    icon: Container,
    title: "Container Management",
    description:
      "Start, stop, restart, and remove containers with a single click. View detailed information including environment variables, volumes, and ports.",
    number: "02",
  },
  {
    icon: Layers,
    title: "Beautiful Interface",
    description:
      "Enjoy a modern, intuitive UI with dark and light mode support. Built with React and Tailwind CSS for a smooth, responsive experience.",
    number: "03",
  },
  {
    icon: Zap,
    title: "Lightning Fast",
    description:
      "Built with Go and optimized for performance. Single binary deployment with embedded frontend. No complex setup required.",
    number: "04",
  },
  {
    icon: Shield,
    title: "Optional Authentication",
    description:
      "Secure your instance with JWT-based authentication or run completely open. Read-only mode available for safe viewing without modifications.",
    number: "05",
  },
  {
    icon: Download,
    title: "Easy Installation",
    description:
      "Deploy with Docker Compose in seconds. Single binary with embedded frontend. No database required. Works with any Docker host.",
    number: "06",
  },
];

const containerVariants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
    },
  },
};

export function Features() {
  return (
    <section id="features" className="container py-20 md:py-32">
      <div className="mx-auto flex max-w-232 flex-col items-center space-y-4 text-center">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5 }}
        >
          <Badge className="mb-4">Features</Badge>
          <h2 className="text-3xl font-bold leading-[1.1] sm:text-4xl md:text-5xl">
            Everything you need to manage Docker
          </h2>
          {/* <p className="mt-4 text-lg text-muted-foreground sm:text-xl">
            LogDeck provides a comprehensive set of features to help you monitor
            and manage your Docker containers efficiently.
          </p> */}
        </motion.div>
      </div>

      <motion.div
        variants={containerVariants}
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: "-100px" }}
        className="mx-auto mt-16 grid max-w-7xl gap-6 sm:grid-cols-2 lg:grid-cols-3"
      >
        {features.map((feature) => {
          const Icon = feature.icon;
          return (
            <motion.div key={feature.title} variants={itemVariants}>
              <Card className="relative h-full overflow-hidden border-border/50 transition-all hover:border-border hover:shadow-lg">
                <div className="absolute top-4 right-4 text-8xl font-bold text-muted/5">
                  {feature.number}
                </div>
                <CardHeader>
                  <div className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                    <Icon className="h-6 w-6 text-primary" />
                  </div>
                  <CardTitle className="text-xl">{feature.title}</CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription className="text-base">
                    {feature.description}
                  </CardDescription>
                </CardContent>
              </Card>
            </motion.div>
          );
        })}
      </motion.div>
    </section>
  );
}

function Badge({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`inline-flex items-center rounded-lg bg-muted px-3 py-1 text-sm font-medium ${className}`}
    >
      {children}
    </div>
  );
}
