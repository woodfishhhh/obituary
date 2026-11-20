# Política de firma de código

[SignPath.io](https://about.signpath.io/) ofrece la firma de código de forma gratuita. [SignPath Foundation](https://signpath.org/) proporciona el certificado.

## Lo que se firma

- Los ejecutables `obit.exe`, `obitd.exe` y `obit-gui.exe` incluidos en los archivos comprimidos de cada versión.
- El flujo de publicación de GitHub Actions ([`release.yml`](https://github.com/woodfishhhh/obituary/blob/main/.github/workflows/release.yml)) compila estos programas automáticamente y solo a partir de commits etiquetados de este repositorio. Los scripts de compilación y la configuración de CI se encuentran en el mismo repositorio.
- Cada solicitud de firma requiere la aprobación manual de una de las personas que figuran a continuación.
- En Propiedades → Detalles figuran el nombre del producto «Obituary 讣告» y la versión.

## Funciones del equipo

| Función | Miembros |
| --- | --- |
| Responsables de incorporar y revisar cambios | [woodfishhhh](https://github.com/woodfishhhh) |
| Responsables de aprobar las firmas | [woodfishhhh](https://github.com/woodfishhhh) |

Un mantenedor revisa cada cambio que envía una persona ajena al equipo antes de integrarlo. Todos los miembros usan autenticación multifactor en GitHub.

## Privacidad

Puedes desactivar por separado las estadísticas de uso y la búsqueda automática de actualizaciones. Consulta la [política de privacidad](privacy).
