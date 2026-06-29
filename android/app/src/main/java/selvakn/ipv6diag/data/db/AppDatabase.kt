package selvakn.ipv6diag.data.db

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase
import androidx.room.TypeConverter
import androidx.room.TypeConverters
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json

val MIGRATION_1_2 = object : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL(
            """CREATE TABLE IF NOT EXISTS xlat_summaries (
                session_id TEXT NOT NULL PRIMARY KEY,
                summary_json TEXT NOT NULL,
                overall_status TEXT NOT NULL,
                FOREIGN KEY(session_id) REFERENCES diagnostic_sessions(id) ON DELETE CASCADE
            )"""
        )
    }
}

val MIGRATION_2_3 = object : Migration(2, 3) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE server_endpoints ADD COLUMN use_https INTEGER NOT NULL DEFAULT 0")
    }
}

val MIGRATION_3_4 = object : Migration(3, 4) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE diagnostic_sessions ADD COLUMN test_endpoint_host TEXT NOT NULL DEFAULT ''")
    }
}

@Database(
    entities = [
        DiagnosticSessionEntity::class,
        TestResultEntity::class,
        ServerEndpointEntity::class,
        XlatSummaryEntity::class,
    ],
    version = 4,
    exportSchema = false,
)
@TypeConverters(Converters::class)
abstract class AppDatabase : RoomDatabase() {
    abstract fun sessionDao(): DiagnosticSessionDao
    abstract fun testResultDao(): TestResultDao
    abstract fun serverEndpointDao(): ServerEndpointDao
    abstract fun xlatSummaryDao(): XlatSummaryDao

    companion object {
        @Volatile private var INSTANCE: AppDatabase? = null

        fun getInstance(context: Context): AppDatabase =
            INSTANCE ?: synchronized(this) {
                INSTANCE ?: Room.databaseBuilder(
                    context.applicationContext,
                    AppDatabase::class.java,
                    "ipv6diag.db",
                ).addMigrations(MIGRATION_1_2, MIGRATION_2_3, MIGRATION_3_4).build().also { INSTANCE = it }
            }
    }
}

class Converters {
    private val json = Json { ignoreUnknownKeys = true }

    @TypeConverter
    fun listToJson(value: List<String>): String = json.encodeToString(ListSerializer(String.serializer()), value)

    @TypeConverter
    fun jsonToList(value: String): List<String> =
        json.decodeFromString(value)
}
