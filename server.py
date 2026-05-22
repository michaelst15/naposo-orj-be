from fastapi import FastAPI, APIRouter, HTTPException
from dotenv import load_dotenv
from starlette.middleware.cors import CORSMiddleware
import asyncpg
import os
import logging
from pathlib import Path
from pydantic import BaseModel, Field, ConfigDict
from typing import List, Optional, Any, Dict
import uuid
from datetime import datetime, timezone


ROOT_DIR = Path(__file__).parent
load_dotenv(ROOT_DIR / ".env")

app = FastAPI()
api_router = APIRouter(prefix="/api")


class Member(BaseModel):
    model_config = ConfigDict(extra="ignore")
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    nama: str
    angkatan: str
    posisi: str
    foto: str
    bio: str
    kontak: str


class MemberCreate(BaseModel):
    nama: str
    angkatan: str
    posisi: str
    foto: str
    bio: str
    kontak: str


class Pengurus(BaseModel):
    model_config = ConfigDict(extra="ignore")
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    nama: str
    posisi: str
    foto: str
    bio: str
    urutan: int


class PengurusCreate(BaseModel):
    nama: str
    posisi: str
    foto: str
    bio: str
    urutan: int


class Activity(BaseModel):
    model_config = ConfigDict(extra="ignore")
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    judul: str
    deskripsi: str
    tanggal: str
    status: str
    gambar: str


class ActivityCreate(BaseModel):
    judul: str
    deskripsi: str
    tanggal: str
    status: str
    gambar: str


class Gallery(BaseModel):
    model_config = ConfigDict(extra="ignore")
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    judul: str
    gambar: str
    tanggal: str
    deskripsi: str


class GalleryCreate(BaseModel):
    judul: str
    gambar: str
    tanggal: str
    deskripsi: str


class Registration(BaseModel):
    model_config = ConfigDict(extra="ignore")
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    nama: str
    email: str
    telepon: str
    angkatan: str
    alasan: str
    tanggal_daftar: str = Field(default_factory=lambda: datetime.now(timezone.utc).isoformat())


class RegistrationCreate(BaseModel):
    nama: str
    email: str
    telepon: str
    angkatan: str
    alasan: str


def _row_to_dict(row: Any) -> Dict[str, Any]:
    data = dict(row)
    if "tanggal_daftar" in data and isinstance(data["tanggal_daftar"], datetime):
        data["tanggal_daftar"] = data["tanggal_daftar"].isoformat()
    return data


async def _ensure_schema(conn: asyncpg.Connection) -> None:
    await conn.execute(
        """
        CREATE TABLE IF NOT EXISTS members (
          id TEXT PRIMARY KEY,
          nama TEXT NOT NULL,
          angkatan TEXT NOT NULL,
          posisi TEXT NOT NULL,
          foto TEXT NOT NULL,
          bio TEXT NOT NULL,
          kontak TEXT NOT NULL
        )
        """
    )
    await conn.execute(
        """
        CREATE TABLE IF NOT EXISTS pengurus (
          id TEXT PRIMARY KEY,
          nama TEXT NOT NULL,
          posisi TEXT NOT NULL,
          foto TEXT NOT NULL,
          bio TEXT NOT NULL,
          urutan INTEGER NOT NULL
        )
        """
    )
    await conn.execute(
        """
        CREATE TABLE IF NOT EXISTS activities (
          id TEXT PRIMARY KEY,
          judul TEXT NOT NULL,
          deskripsi TEXT NOT NULL,
          tanggal TEXT NOT NULL,
          status TEXT NOT NULL,
          gambar TEXT NOT NULL
        )
        """
    )
    await conn.execute(
        """
        CREATE TABLE IF NOT EXISTS gallery (
          id TEXT PRIMARY KEY,
          judul TEXT NOT NULL,
          gambar TEXT NOT NULL,
          tanggal TEXT NOT NULL,
          deskripsi TEXT NOT NULL
        )
        """
    )
    await conn.execute(
        """
        CREATE TABLE IF NOT EXISTS registrations (
          id TEXT PRIMARY KEY,
          nama TEXT NOT NULL,
          email TEXT NOT NULL,
          telepon TEXT NOT NULL,
          angkatan TEXT NOT NULL,
          alasan TEXT NOT NULL,
          tanggal_daftar TIMESTAMPTZ NOT NULL DEFAULT now()
        )
        """
    )


@app.on_event("startup")
async def startup() -> None:
    database_url = os.environ.get("DATABASE_URL", "").strip().strip('"').strip("'")
    if not database_url:
        raise RuntimeError("DATABASE_URL belum di-set. Isi app/backend/.env dengan connection string PostgreSQL.")

    ssl = "require" if "sslmode=require" in database_url else None
    app.state.db_pool = await asyncpg.create_pool(dsn=database_url, ssl=ssl, min_size=1, max_size=5)

    async with app.state.db_pool.acquire() as conn:
        await _ensure_schema(conn)


@app.on_event("shutdown")
async def shutdown() -> None:
    pool = getattr(app.state, "db_pool", None)
    if pool:
        await pool.close()


@api_router.get("/")
async def root():
    return {"message": "Naposo ORJ API"}


@api_router.get("/members", response_model=List[Member])
async def get_members(angkatan: Optional[str] = None, posisi: Optional[str] = None, search: Optional[str] = None):
    where = []
    args: List[Any] = []

    if angkatan:
        args.append(angkatan)
        where.append(f"angkatan = ${len(args)}")
    if posisi:
        args.append(posisi)
        where.append(f"posisi = ${len(args)}")
    if search:
        args.append(f"%{search}%")
        where.append(f"nama ILIKE ${len(args)}")

    sql = "SELECT id, nama, angkatan, posisi, foto, bio, kontak FROM members"
    if where:
        sql += " WHERE " + " AND ".join(where)

    async with app.state.db_pool.acquire() as conn:
        rows = await conn.fetch(sql, *args)
    return [_row_to_dict(r) for r in rows]


@api_router.post("/members", response_model=Member)
async def create_member(member: MemberCreate):
    member_obj = Member(**member.model_dump())
    async with app.state.db_pool.acquire() as conn:
        await conn.execute(
            """
            INSERT INTO members (id, nama, angkatan, posisi, foto, bio, kontak)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
            """,
            member_obj.id,
            member_obj.nama,
            member_obj.angkatan,
            member_obj.posisi,
            member_obj.foto,
            member_obj.bio,
            member_obj.kontak,
        )
    return member_obj


@api_router.delete("/members/{member_id}")
async def delete_member(member_id: str):
    async with app.state.db_pool.acquire() as conn:
        result = await conn.execute("DELETE FROM members WHERE id = $1", member_id)
    deleted = int(result.split()[-1]) if result else 0
    if deleted == 0:
        raise HTTPException(status_code=404, detail="Member not found")
    return {"message": "Member deleted"}


@api_router.get("/pengurus", response_model=List[Pengurus])
async def get_pengurus():
    async with app.state.db_pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT id, nama, posisi, foto, bio, urutan FROM pengurus ORDER BY urutan ASC"
        )
    return [_row_to_dict(r) for r in rows]


@api_router.post("/pengurus", response_model=Pengurus)
async def create_pengurus(pengurus: PengurusCreate):
    pengurus_obj = Pengurus(**pengurus.model_dump())
    async with app.state.db_pool.acquire() as conn:
        await conn.execute(
            """
            INSERT INTO pengurus (id, nama, posisi, foto, bio, urutan)
            VALUES ($1, $2, $3, $4, $5, $6)
            """,
            pengurus_obj.id,
            pengurus_obj.nama,
            pengurus_obj.posisi,
            pengurus_obj.foto,
            pengurus_obj.bio,
            pengurus_obj.urutan,
        )
    return pengurus_obj


@api_router.get("/activities", response_model=List[Activity])
async def get_activities(status: Optional[str] = None):
    sql = "SELECT id, judul, deskripsi, tanggal, status, gambar FROM activities"
    args: List[Any] = []
    if status:
        args.append(status)
        sql += f" WHERE status = ${len(args)}"

    async with app.state.db_pool.acquire() as conn:
        rows = await conn.fetch(sql, *args)
    return [_row_to_dict(r) for r in rows]


@api_router.post("/activities", response_model=Activity)
async def create_activity(activity: ActivityCreate):
    activity_obj = Activity(**activity.model_dump())
    async with app.state.db_pool.acquire() as conn:
        await conn.execute(
            """
            INSERT INTO activities (id, judul, deskripsi, tanggal, status, gambar)
            VALUES ($1, $2, $3, $4, $5, $6)
            """,
            activity_obj.id,
            activity_obj.judul,
            activity_obj.deskripsi,
            activity_obj.tanggal,
            activity_obj.status,
            activity_obj.gambar,
        )
    return activity_obj


@api_router.get("/gallery", response_model=List[Gallery])
async def get_gallery():
    async with app.state.db_pool.acquire() as conn:
        rows = await conn.fetch("SELECT id, judul, gambar, tanggal, deskripsi FROM gallery")
    return [_row_to_dict(r) for r in rows]


@api_router.post("/gallery", response_model=Gallery)
async def create_gallery(gallery: GalleryCreate):
    gallery_obj = Gallery(**gallery.model_dump())
    async with app.state.db_pool.acquire() as conn:
        await conn.execute(
            """
            INSERT INTO gallery (id, judul, gambar, tanggal, deskripsi)
            VALUES ($1, $2, $3, $4, $5)
            """,
            gallery_obj.id,
            gallery_obj.judul,
            gallery_obj.gambar,
            gallery_obj.tanggal,
            gallery_obj.deskripsi,
        )
    return gallery_obj


@api_router.post("/registrations", response_model=Registration)
async def create_registration(registration: RegistrationCreate):
    reg_obj = Registration(**registration.model_dump())
    async with app.state.db_pool.acquire() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO registrations (id, nama, email, telepon, angkatan, alasan)
            VALUES ($1, $2, $3, $4, $5, $6)
            RETURNING id, nama, email, telepon, angkatan, alasan, tanggal_daftar
            """,
            reg_obj.id,
            reg_obj.nama,
            reg_obj.email,
            reg_obj.telepon,
            reg_obj.angkatan,
            reg_obj.alasan,
        )
    return _row_to_dict(row)


@api_router.get("/registrations", response_model=List[Registration])
async def get_registrations():
    async with app.state.db_pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT id, nama, email, telepon, angkatan, alasan, tanggal_daftar FROM registrations ORDER BY tanggal_daftar DESC"
        )
    return [_row_to_dict(r) for r in rows]


@api_router.post("/seed")
async def seed_data():
    async with app.state.db_pool.acquire() as conn:
        async with conn.transaction():
            await _ensure_schema(conn)
            await conn.execute("TRUNCATE TABLE members, pengurus, activities, gallery, registrations")

            pengurus_rows = [
                (str(uuid.uuid4()), "Andi Pratama", "Ketua", "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=400", "Memimpin dengan visi membawa perubahan positif", 1),
                (str(uuid.uuid4()), "Siti Nurhaliza", "Wakil Ketua", "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=400", "Berdedikasi untuk kemajuan organisasi", 2),
                (str(uuid.uuid4()), "Budi Santoso", "Sekretaris", "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=400", "Mengatur administrasi dengan rapi dan terstruktur", 3),
                (str(uuid.uuid4()), "Dewi Lestari", "Bendahara", "https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=400", "Mengelola keuangan dengan transparan", 4),
                (str(uuid.uuid4()), "Rizky Firmansyah", "Koordinator Acara", "https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?w=400", "Menciptakan event seru dan berkesan", 5),
                (str(uuid.uuid4()), "Maya Indah", "Koordinator Media", "https://images.unsplash.com/photo-1517841905240-472988babdf9?w=400", "Mengelola konten digital dan publikasi", 6),
            ]
            await conn.executemany(
                """
                INSERT INTO pengurus (id, nama, posisi, foto, bio, urutan)
                VALUES ($1, $2, $3, $4, $5, $6)
                """,
                pengurus_rows,
            )

            members_rows = [
                (str(uuid.uuid4()), "Ahmad Fadhil", "2020", "Anggota Aktif", "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=400", "Senang berbagi dan belajar hal baru", "ahmad.fadhil@email.com"),
                (str(uuid.uuid4()), "Linda Wijaya", "2021", "Anggota Aktif", "https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=400", "Passionate tentang social impact", "linda.wijaya@email.com"),
                (str(uuid.uuid4()), "Dimas Ardianto", "2021", "Volunteer", "https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?w=400", "Suka membantu dan berkolaborasi", "dimas.ardianto@email.com"),
                (str(uuid.uuid4()), "Putri Ayu", "2022", "Anggota Baru", "https://images.unsplash.com/photo-1487412720507-e7ab37603c6f?w=400", "Excited untuk berkontribusi!", "putri.ayu@email.com"),
                (str(uuid.uuid4()), "Arief Rahman", "2020", "Volunteer", "https://images.unsplash.com/photo-1492562080023-ab3db95bfbce?w=400", "Believe in community power", "arief.rahman@email.com"),
                (str(uuid.uuid4()), "Fatimah Zahra", "2022", "Anggota Aktif", "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=400", "Love connecting with people", "fatimah.zahra@email.com"),
                (str(uuid.uuid4()), "Hendra Gunawan", "2021", "Anggota Aktif", "https://images.unsplash.com/photo-1463453091185-61582044d556?w=400", "Passionate about youth empowerment", "hendra.gunawan@email.com"),
                (str(uuid.uuid4()), "Raisa Andriana", "2022", "Anggota Baru", "https://images.unsplash.com/photo-1489424731084-a5d8b219a5bb?w=400", "Ready to make an impact", "raisa.andriana@email.com"),
            ]
            await conn.executemany(
                """
                INSERT INTO members (id, nama, angkatan, posisi, foto, bio, kontak)
                VALUES ($1, $2, $3, $4, $5, $6, $7)
                """,
                members_rows,
            )

            activities_rows = [
                (str(uuid.uuid4()), "Gathering Bulanan", "Pertemuan rutin bulanan untuk sharing dan networking", "2026-02-15", "upcoming", "https://images.unsplash.com/photo-1511988617509-a57c8a288659?w=800"),
                (str(uuid.uuid4()), "Workshop Leadership", "Pelatihan kepemimpinan untuk pengembangan diri", "2026-02-28", "upcoming", "https://images.unsplash.com/photo-1573497701240-345a300b8d36?w=800"),
                (str(uuid.uuid4()), "Bakti Sosial", "Kegiatan sosial membantu masyarakat sekitar", "2026-01-20", "past", "https://images.unsplash.com/photo-1559027615-cd4628902d4a?w=800"),
                (str(uuid.uuid4()), "Fun Games Day", "Hari penuh permainan seru dan team building", "2026-01-10", "past", "https://images.unsplash.com/photo-1702599057905-d3859caa8b61?w=800"),
                (str(uuid.uuid4()), "Seminar Karir", "Tips dan trik membangun karir di era digital", "2026-03-10", "upcoming", "https://images.unsplash.com/photo-1540575467063-178a50c2df87?w=800"),
            ]
            await conn.executemany(
                """
                INSERT INTO activities (id, judul, deskripsi, tanggal, status, gambar)
                VALUES ($1, $2, $3, $4, $5, $6)
                """,
                activities_rows,
            )

            gallery_rows = [
                (str(uuid.uuid4()), "Gathering Perdana 2025", "https://images.unsplash.com/photo-1511988617509-a57c8a288659?w=800", "2025-12-15", "Pertemuan pertama tahun 2025"),
                (str(uuid.uuid4()), "Workshop Kreatif", "https://images.unsplash.com/photo-1573497701240-345a300b8d36?w=800", "2025-11-20", "Pelatihan kreativitas bersama"),
                (str(uuid.uuid4()), "Outing Bersama", "https://images.unsplash.com/photo-1702599057905-d3859caa8b61?w=800", "2025-10-10", "Refreshing di alam terbuka"),
                (str(uuid.uuid4()), "Diskusi Komunitas", "https://images.unsplash.com/photo-1590650046871-92c887180603?w=800", "2025-12-05", "Sharing session tentang isu sosial"),
            ]
            await conn.executemany(
                """
                INSERT INTO gallery (id, judul, gambar, tanggal, deskripsi)
                VALUES ($1, $2, $3, $4, $5)
                """,
                gallery_rows,
            )

    return {"message": "Data seeded successfully"}


app.include_router(api_router)

app.add_middleware(
    CORSMiddleware,
    allow_credentials=True,
    allow_origins=os.environ.get("CORS_ORIGINS", "*").split(","),
    allow_methods=["*"],
    allow_headers=["*"],
)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

