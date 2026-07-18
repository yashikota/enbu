import { useEffect } from "react";
import { Box, HStack } from "styled-system/jsx";
import { Button, Heading } from "./ui";
import { Trash2 } from "lucide-react";

export function ConfirmDeleteDialog({
  open,
  title,
  cancelLabel,
  confirmLabel,
  loading,
  onClose,
  onConfirm,
}: {
  open: boolean;
  title: string;
  cancelLabel: string;
  confirmLabel: string;
  loading: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
}) {
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !loading) onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [loading, onClose, open]);

  if (!open) return null;
  return (
    <Box
      position="fixed"
      inset="0"
      zIndex="60"
      display="grid"
      placeItems="center"
      p="4"
      bg="rgba(15, 23, 42, 0.48)"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && !loading) onClose();
      }}
    >
      <Box
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-delete-title"
        w="full"
        maxW="440px"
        p="5"
        bg="bg.surface"
        borderWidth="1px"
        borderColor="border.default"
        borderRadius="2xl"
        boxShadow="xl"
      >
        <Heading id="confirm-delete-title" size="lg" fontWeight="extrabold">
          {title}
        </Heading>
        <HStack justify="end" gap="2" mt="6">
          <Button type="button" variant="ghost" disabled={loading} onClick={onClose}>
            {cancelLabel}
          </Button>
          <Button
            type="button"
            bg="status.danger"
            color="white"
            loading={loading}
            aria-label={`${confirmLabel}: ${title}`}
            onClick={() => void onConfirm()}
          >
            <Trash2 size={15} />
            {confirmLabel}
          </Button>
        </HStack>
      </Box>
    </Box>
  );
}
