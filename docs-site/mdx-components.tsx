import { useMDXComponents as getDocsMDXComponents } from "nextra-theme-docs"
import { Clip, Premium } from "@/components/Clip"

const docsComponents = getDocsMDXComponents()

export function useMDXComponents(components?: Record<string, React.ComponentType>) {
  return {
    ...docsComponents,
    Clip,
    Premium,
    ...components,
  }
}
