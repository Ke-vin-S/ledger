import type { CodegenConfig } from "@graphql-codegen/cli";

const config: CodegenConfig = {
  overwrite: true,
  schema: "http://localhost:8080/graphql",
  documents: ["lib/graphql/**/*.ts", "!lib/graphql/types.ts"],
  generates: {
    "lib/graphql/types.ts": {
      plugins: ["typescript", "typescript-operations"],
      config: {
        scalars: {
          Map: "Record<string, unknown>",
        },
      },
    },
  },
};

export default config;
