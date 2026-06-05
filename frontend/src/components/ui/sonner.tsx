import { CheckCircle, Info, XCircle, AlertTriangle, RefreshCw } from "lucide-react"
import { Toaster as Sonner } from "sonner"

type ToasterProps = React.ComponentProps<typeof Sonner>

const Toaster = ({ ...props }: ToasterProps) => {
  return (
    <Sonner
      theme="dark"
      className="toaster group"
      icons={{
        success: <CheckCircle className="size-4" />,
        info: <Info className="size-4" />,
        warning: <AlertTriangle className="size-4" />,
        error: <XCircle className="size-4" />,
        loading: <RefreshCw className="size-4 animate-spin" />,
      }}
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-overlay group-[.toaster]:text-fg group-[.toaster]:border-border group-[.toaster]:shadow-lg",
          description: "group-[.toast]:text-muted-fg",
          actionButton:
            "group-[.toast]:bg-primary group-[.toast]:text-primary-fg",
          cancelButton:
            "group-[.toast]:bg-muted group-[.toast]:text-muted-fg",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
