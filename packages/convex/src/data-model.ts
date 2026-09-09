import type { DataModelFromSchemaDefinition } from "convex/server"
import type schema from "../convex/schema"

export type DataModel = DataModelFromSchemaDefinition<typeof schema>
